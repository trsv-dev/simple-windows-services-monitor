package main

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/router"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/server"
)

func main() {
	logger.InitLogger()

	srvConfig := config.InitConfig()
	srvRouter := router.Router()

	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()

	err := server.RunServer(srvConfig.RunAddress, srvRouter)
	if err != nil {
		panic(err)
	}
}
