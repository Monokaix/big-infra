package test

import (
	"context"

	v1 "big-infra/pkg/apiserver/api/v1"
)

var InfraCli *InfraGrpcClient

const (
	grpcAddr = "127.0.0.1:5000"
)

type InfraGrpcClient struct {
	cli v1.INFRAAPPLYClient
	ctx context.Context
}
