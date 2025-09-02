package main

import (
	"os"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/server"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/postgres"
)

func main() {
	srvConfig := config.InitConfig()

	logger.InitLogger(srvConfig.LogLevel)

	storage, err := postgres.InitStorage(srvConfig.DatabaseURI)
	if err != nil {
		logger.Log.Error("Не удалось инициировать хранилище (БД)", logger.String("err", err.Error()))
		os.Exit(1)
	}

	err = server.RunServer(srvConfig.RunAddress, storage)
	if err != nil {
		panic(err)
	}
}
