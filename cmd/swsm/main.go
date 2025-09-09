package main

import (
	"encoding/base64"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/server"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/postgres"
)

func main() {
	errEnv := godotenv.Load("../../.env")
	if errEnv != nil {
		log.Println("Не удалось загрузить .env:", errEnv)
	}

	srvConfig := config.InitConfig()

	logger.InitLogger(srvConfig.LogLevel)

	AESKeyStr := srvConfig.AESKey
	AESKeyBytes, err := base64.StdEncoding.DecodeString(AESKeyStr)
	if err != nil {
		logger.Log.Error("Не удалось декодировать AES-ключ из конфигурации", logger.String("err", err.Error()))
		os.Exit(1)
	}

	storage, err := postgres.InitStorage(srvConfig.DatabaseURI, AESKeyBytes)
	if err != nil {
		logger.Log.Error("Не удалось инициировать хранилище (БД)", logger.String("err", err.Error()))
		os.Exit(1)
	}

	err = server.RunServer(srvConfig.RunAddress, storage)
	if err != nil {
		panic(err)
	}
}
