package main

import (
	"context"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc"
	"net/http"
	"os"

	v1 "big-infra/pkg/apiserver/api/v1"
	"big-infra/pkg/apiserver/common"
	"big-infra/pkg/apiserver/config"
	"big-infra/pkg/apiserver/service"

	logger "github.com/sirupsen/logrus"
)

const (
	grpcPort = ":5000"
	httpPort = ":8080"
)

func main() {
	logger.Info()
	logger.Info("starting apiserver for TuPam")

	confPath := os.Getenv("API_SRV_CONF_PATH")
	if confPath == "" {
		// dev mode, configure path load from file
		logger.Info("load configure from file")
		confPath = common.CONF_PATH
	}

	env, err := config.Init(confPath)
	if err != nil {
		logger.Panic(err)
	}

	s := service.New(env)

	go s.SignalHandler()
	clientAddr := fmt.Sprintf("localhost%s", grpcPort)
	go StartHTTPServer(httpPort, clientAddr)
	if err := s.Start(env.Cfg.GrpcSrv.Address); err != nil {
		logger.Panic(err)
	}
}

// start the http server
func StartHTTPServer(addr, clientAddr string) {
	logger.Info("Starting HTTP Server...")

	opts := []grpc.DialOption{grpc.WithInsecure()}
	mux := runtime.NewServeMux()
	if err := v1.RegisterINFRAAPPLYHandlerFromEndpoint(context.Background(), mux, clientAddr, opts); err != nil {
		logger.Fatalf("failed to start HTTP server: %v", err)
	}
	logger.Info("HTTP Listening on %s", addr)
	logger.Fatal(http.ListenAndServe(addr, mux))
}
