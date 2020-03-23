package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	v1 "big-infra/pkg/apiserver/api/v1"
	"big-infra/pkg/apiserver/config"
	"big-infra/pkg/apiserver/server"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	logger "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	_abortIndex  int8 = math.MaxInt8 / 2
	_traceID          = "trace_id"
	_uid              = "uid"
	_token            = "token"
	_headerAuthz      = "authorization"
	_bearer           = "Bearer"
)

var (
	_whitelist = []string{
	}
)

// TuPam grpc service struct
type InfraApplyServiceV1 struct {
	env *config.Env
	uid string
}

// GrpcService is the grpc server and its configurations.
type GrpcService struct {
	env      *config.Env
	server   *grpc.Server
	handlers []grpc.UnaryServerInterceptor
}

type BasiceClaim struct {
	UID       string `json:"uid"`
	ExpiresAt int64  `json:"exp"`
}

func (c BasiceClaim) Valid() error {
	vErr := new(jwt.ValidationError)
	now := time.Now().Unix()
	if c.ExpiresAt == 0 {
		vErr.Inner = fmt.Errorf("exp is required")
		vErr.Errors |= jwt.ValidationErrorClaimsInvalid
	}
	if c.ExpiresAt < now {
		delta := time.Unix(now, 0).Sub(time.Unix(c.ExpiresAt, 0))
		vErr.Inner = fmt.Errorf("token is expired by %v", delta)
		vErr.Errors |= jwt.ValidationErrorExpired
	}

	if c.UID == "" {
		vErr.Inner = fmt.Errorf("uid is required")
		vErr.Errors |= jwt.ValidationErrorClaimsInvalid
	}

	if vErr.Errors == 0 {
		return nil
	}

	return vErr
}

// New news a GrpcService using customized configurations.
func New(env *config.Env, opt ...grpc.ServerOption) *GrpcService {
	keepAlive := grpc.KeepaliveParams(keepalive.ServerParameters{
		MaxConnectionIdle:     time.Duration(time.Second * 60),
		MaxConnectionAge:      time.Duration(time.Hour * 2),
		MaxConnectionAgeGrace: time.Duration(time.Second * 20),
		Time:                  time.Duration(time.Second * 60),
		Timeout:               time.Duration(time.Second * 5),
	})

	s := new(GrpcService)
	s.env = env

	opt = append(opt, keepAlive, grpc.UnaryInterceptor(s.interceptor))

	s.server = grpc.NewServer(opt...)
	s.Use(s.recovery(), s.handle(), s.logging())

	v1.RegisterINFRAAPPLYServer(s.server, &InfraApplyServiceV1{env: env})

	return s
}

// Start starts the grpc server.
func (s *GrpcService) Start(address string) error {
	logger.Infof("starting grpc service at: %s", address)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		logger.Panic(err)
		return err
	}

	reflection.Register(s.server)
	return s.server.Serve(listener)
}

// Stop stops the grpc server.
func (s *GrpcService) Stop() {
	s.server.Stop()
}

// interceptor is a single interceptor out of a chain of many interceptors.
// Execution is done in left-to-right order, including passing of context.
// For example ChainUnaryServer(one, two, three) will execute one before two before three, and three
// will see context changes of one and two.
func (s *GrpcService) interceptor(ctx context.Context, req interface{},
	args *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	var (
		i     int
		chain grpc.UnaryHandler
	)

	n := len(s.handlers)
	if n == 0 {
		return handler(ctx, req)
	}

	chain = func(ic context.Context, ir interface{}) (interface{}, error) {
		if i == n-1 {
			return handler(ic, ir)
		}
		i++
		return s.handlers[i](ic, ir, args, chain)
	}

	return s.handlers[0](ctx, req, args, chain)
}

// Use attachs a global inteceptor to the server.
// For example, this is the right place for a rate limiter or error management inteceptor
func (s *GrpcService) Use(handlers ...grpc.UnaryServerInterceptor) *GrpcService {
	finalSize := len(s.handlers) + len(handlers)
	if finalSize >= int(_abortIndex) {
		panic("grep service: server use too many handlers")
	}
	mergedHandlers := make([]grpc.UnaryServerInterceptor, finalSize)
	copy(mergedHandlers, s.handlers)
	copy(mergedHandlers[len(s.handlers):], handlers)
	s.handlers = mergedHandlers

	return s
}

// recovery is a server interceptor that recovers from any panics.
func (s *GrpcService) recovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, args *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if rerr := recover(); rerr != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				_ = runtime.Stack(buf, false)
				logger.Errorf("grpc server panic: %v\n%v\n%s\n", req, rerr, buf)
				err = status.Errorf(codes.Unknown, fmt.Sprintf("%v", rerr))
			}
		}()
		resp, err = handler(ctx, req)
		return
	}
}

// tracing, auth 等几个拦截器分开写比较好
// handle return a new unary server interceptor for Tracing\LinkTimeout\AuthToken
func (s *GrpcService) handle() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, args *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// get trace_id from frontend, and set into ctx
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "empty metadata")
		}

		var traceID string

		_, ok = md[_traceID]
		if !ok {
			traceID = uuid.New().String()
			md.Append(_traceID, traceID)
		}

		// 获取 header 的 authorization 字段的值建议用 go-grpc-middleware 里的 auth
		var token string
		if val, ok := md[_headerAuthz]; ok {
			splits := strings.SplitN(val[0], " ", 2)
			if len(splits) < 2 || splits[0] != _bearer {
				return nil, status.Errorf(codes.Unauthenticated, "bad authorization string")
			}

			token = splits[1]
		}

		uid, _, err := parseToken(token, s.env.Cfg.Identify.AuthSecret)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "parse token failed:", err)
		}

		md.Append(_token, token)
		md.Append(_uid, uid)

		newCtx := metadata.NewIncomingContext(ctx, md)

		return handler(newCtx, req)
	}
}

// grpc logging
func (s *GrpcService) logging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (interface{}, error) {
		startTime := time.Now()
		var remoteIP string
		if peerInfo, ok := peer.FromContext(ctx); ok {
			remoteIP = peerInfo.Addr.String()
		}

		var quota float64
		if deadline, ok := ctx.Deadline(); ok {
			quota = time.Until(deadline).Seconds()
		}

		// call server handler
		resp, err := handler(ctx, req)

		duration := time.Since(startTime)
		logFields := logger.Fields{
			"ip":            remoteIP,
			"path":          info.FullMethod,
			"ts":            duration.Seconds(),
			"timeout_quota": quota,
			"args":          req.(fmt.Stringer).String(),
		}

		if err != nil {
			md, ok := metadata.FromIncomingContext(ctx)
			if !ok {
				return nil, status.Errorf(codes.InvalidArgument, "empty metadata")
			}

			var traceID string
			if value, ok := md[_traceID]; ok {
				traceID = value[0]
			}

			logFields[_traceID] = traceID
			logFields["error"] = err.Error()
			logFields["stack"] = fmt.Sprintf("%+v", err)
		}

		logger.WithFields(logFields).Debugf("grpc request:")
		return resp, err
	}
}

// signal handler
func (s *GrpcService) SignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	ch := <-c

	logger.Infof("apiserver get %s signal", ch.String())
	switch ch {
	case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
		logger.Info("apiserver exit")
		s.Stop()
		s.env.SaveProfile()
		s.env.MysqlCli.Close()
		time.Sleep(time.Second)
		return
	case syscall.SIGHUP:
		// TODO reload
	default:
		return
	}
}

func parseToken(tokenStr, authSecret string) (uid string, exp int64, err error) {
	fn := func(token *jwt.Token) (interface{}, error) {
		return []byte(authSecret), nil
	}

	token, err := jwt.ParseWithClaims(tokenStr, &BasiceClaim{}, fn)

	if err != nil {
		return
	}

	claim, ok := token.Claims.(*BasiceClaim)
	if !ok {
		err = errors.New("cannot convert claim to BasicClaim")
		return
	}

	uid = claim.UID
	exp = claim.ExpiresAt
	return uid, exp, nil
}

// GetUser is InfraApplyServiceV1's internal interface
func (h *InfraApplyServiceV1) GetUser(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx) // ignore the `error` return value
	return md[_uid][0]
}

// List
func (s *InfraApplyServiceV1) ListInfraApply(ctx context.Context, in *v1.ListInfraApplyReq) (*v1.ListInfraApplyReply, error) {
	pageIdx, pageSize := in.PageIdx-1, in.PageSize
	var limit, offset int32 = pageSize, pageSize * pageIdx

	query := make(map[string]interface{})

	search := make(map[string]interface{})
	if in.Search != "" {
		search["subject_name"] = in.Search
	}

	res, total, err := server.FindInfraApplyLikePattern(s.env.MysqlCli, query, search, limit, offset)
	if err != nil {
		return nil, err
	}
	ret := v1.ListInfraApplyReply{}
	for _, ia := range res {
		record := v1.DetailInfraApplyReply{
			ID:          ia.ID,
			DeviceCode:  ia.DeviceCode,
			Applyer:     ia.Applyer,
			Status:      ia.Status,
			SubjectName: ia.SubjectName,
			ReviewId:    ia.ReviewId,
			ExpireTM:    ia.ExpiresAt.String(),
			ReviewTM:    ia.ReviewedAt.String(),
		}
		ret.Record = append(ret.Record, &record)
	}
	ret.Page = &v1.ModelPage{PageSize: pageSize, PageIdx: pageIdx + 1, Total: int32(total)}
	if (limit + offset) < int32(total) {
		ret.Exhausted = false
	} else {
		ret.Exhausted = true
	}
	return &ret, nil

}

func (s *InfraApplyServiceV1) AddInfraApply(ctx context.Context, in *v1.AddInfraApplyReq) (*v1.AddInfraApplyReply, error) {
	return &v1.AddInfraApplyReply{}, nil
}

func (s *InfraApplyServiceV1) UpdateInfraApply(ctx context.Context, in *v1.UpdateInfraApplyReq) (*v1.UpdateInfraApplyReply, error) {
	return &v1.UpdateInfraApplyReply{}, nil
}

func (s *InfraApplyServiceV1) DelInfraApply(ctx context.Context, in *v1.DelInfraApplyReq) (*v1.DelInfraApplyReply, error) {
	return &v1.DelInfraApplyReply{}, nil
}
