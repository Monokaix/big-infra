package test

import (
	"context"
	"os"
	"testing"

	v1 "big-infra/pkg/apiserver/api/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestMain(m *testing.M) {
	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := v1.NewINFRAAPPLYClient(conn)
	md := metadata.Pairs("authorization", "")
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	InfraCli = &InfraGrpcClient{
		cli: client,
		ctx: ctx,
	}
	os.Exit(m.Run())
}
