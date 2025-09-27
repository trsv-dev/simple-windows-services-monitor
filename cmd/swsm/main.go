package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
	// recover для логирования паник
	defer func() {
		if r := recover(); r != nil {
			logger.Log.Error("Паника в main", logger.String("panic", fmt.Sprintf("%v", r)))
		}
	}()

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
	//broadcaster := broadcast.NewR3labsSSEAdapter()
	broadcaster := broadcast.NewR3labsSSEAdapter(
		broadcast.MakeJWTTopicResolver(srvConfig.JWTSecretKey),
	)

	// создаём AppHandler — контейнер зависимостей для всех хендлеров,
	// передаём в него хранилище, JWT ключ и SSE адаптер
	appHandler := api.NewAppHandler(storage, srvConfig.JWTSecretKey, broadcaster)

	// запуск HTTP-сервера,
	// передаём готовый AppHandler, содержащий все зависимости

	// создаем сервер и запускаем его
	srv, serverErrorCh := server.RunServer(srvConfig.RunAddress, appHandler)

	// запускаем воркер в отдельной горутине,
	// воркер периодически опрашивает БД и публикует обновления статусов служб через SSE
	workerCtx, workerCtxCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	var interval time.Duration = 5 * time.Second

	// Запуск status worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.StatusWorker(workerCtx, storage, broadcaster, interval)
	}()

	// канал системных сигналов
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop) // гарантированно перестанем слушать сигнал при выходе

	// блокируемся тут в ожидании одного из вариантов завершения работы сервера
	select {
	case err, ok := <-serverErrorCh:
		if !ok {
			logger.Log.Info("Канал ошибок сервера закрыт")
			return
		}
		logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
	case sig := <-stop:
		logger.Log.Info("Получен сигнал остановки приложения", logger.String("sig", sig.String()))
	}

	logger.Log.Info("Начало процедуры остановки приложения...")

	// если произошло какое-то событие из select выше, считаем что сервер остановлен
	// и останавливаем остальные части приложения:

	// останавливаем status worker
	logger.Log.Info("Остановка status worker...")
	workerCtxCancel()

	// ждем завершения worker с таймаутом
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Log.Info("Status worker остановлен")
	case <-time.After(5 * time.Second):
		logger.Log.Warn("Таймаут остановки status worker")
	}

	// безопасно закрываем broadcaster
	logger.Log.Info("Закрытие broadcaster...")
	if err = broadcaster.Close(); err != nil {
		logger.Log.Warn("Ошибка закрытия SSE адаптера", logger.String("err", err.Error()))
	}

	logger.Log.Info("Успешное закрытие broadcaster")

	// контекст для завершения работы сервера
	serverShutdownCtx, serverShutdownCancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer serverShutdownCancel()

	// остановка сервера
	if err = srv.Shutdown(serverShutdownCtx); err != nil {
		logger.Log.Error("Ошибка остановки сервера", logger.String("err", err.Error()))
	} else {
		logger.Log.Info("Сервер остановлен")
	}

	// закрытие соединения с БД
	logger.Log.Info("Закрытие соединения с БД...")
	if err = storage.Close(); err != nil {
		logger.Log.Error("Ошибка закрытия соединения с БД", logger.String("err", err.Error()))
	}
	logger.Log.Info("Успешное закрытие соединения с БД")

	logger.Log.Info("Приложение завершено")
}
