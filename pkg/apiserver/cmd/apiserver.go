package main

import (
	"os"

	"big-infra/pkg/apiserver/common"
	"big-infra/pkg/apiserver/config"
	"big-infra/pkg/apiserver/service"

	logger "github.com/sirupsen/logrus"
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

	if err := s.Start(env.Cfg.GrpcSrv.Address); err != nil {
		logger.Panic(err)
	}
}
