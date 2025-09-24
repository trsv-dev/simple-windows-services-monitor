package main

import (
	"context"
	"encoding/base64"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/server"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/postgres"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/worker"
)

// "Сборка" и запуск проекта.
func main() {
	// загружаем переменные окружения из .env
	errEnv := godotenv.Load("../../.env")
	if errEnv != nil {
		log.Println("Не удалось загрузить .env:", errEnv)
	}

	// инициализация конфигурации сервера
	srvConfig := config.InitConfig()

	// инициализация логгера с уровнем логирования из конфигурации
	logger.InitLogger(srvConfig.LogLevel)

	// декодируем AES-ключ, используемый для шифрования данных в БД
	AESKeyStr := srvConfig.AESKey
	AESKeyBytes, err := base64.StdEncoding.DecodeString(AESKeyStr)
	if err != nil {
		logger.Log.Error("Не удалось декодировать AES-ключ из конфигурации", logger.String("err", err.Error()))
		os.Exit(1)
	}

	// инициализация хранилища (PostgreSQL) с переданным AES-ключом
	storage, err := postgres.InitStorage(srvConfig.DatabaseURI, AESKeyBytes)
	if err != nil {
		logger.Log.Error("Не удалось инициировать хранилище (БД)", logger.String("err", err.Error()))
		os.Exit(1)
	}

	// создание SSE Publisher/Subscriber,
	// используем r3labs/sse через адаптер, реализующий интерфейс SubscriberManager
	broadcaster := broadcast.NewR3labsSSEAdapter()

	// создаём AppHandler — контейнер зависимостей для всех хендлеров,
	// передаём в него хранилище, JWT ключ и SSE адаптер
	appHandler := api.NewAppHandler(storage, srvConfig.JWTSecretKey, broadcaster)

	// запускаем воркер в отдельной горутине,
	// воркер периодически опрашивает БД и публикует обновления через SSE
	ctx, cancel := context.WithCancel(context.Background())
	var interval time.Duration = 5 * time.Second
	defer cancel()
	go worker.StatusWorker(ctx, storage, broadcaster, interval)

	// запуск HTTP-сервера,
	// передаём готовый AppHandler, содержащий все зависимости
	err = server.RunServer(srvConfig.RunAddress, appHandler)

	if err != nil {
		panic(err)
	}
}
