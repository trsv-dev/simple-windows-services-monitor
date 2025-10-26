package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
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
	// recover для логирования паник в main
	defer func() {
		if r := recover(); r != nil {
			log.Println("Паника в main:", fmt.Sprintf("%v", r))
		}
	}()

	// загружаем переменные окружения из .env для локальной разработки
	errEnv := godotenv.Load("../../.env.development")
	if errEnv != nil {
		log.Println("Не удалось загрузить .env:", errEnv)
	}

	// инициализация конфигурации сервера
	srvConfig := config.InitConfig()

	// инициализация логгера с уровнем логирования из конфигурации
	logger.InitLogger(srvConfig.LogLevel, srvConfig.LogOutput)
	// отложенное закрытие ресурса (актуально если используется файл для логирования)
	defer logger.Log.(*logger.SlogAdapter).Close()

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

	var broadcaster broadcast.Broadcaster

	if srvConfig.WebInterface {
		// создание SSE Publisher/Subscriber,
		// используем r3labs/sse через адаптер, реализующий интерфейс SubscriberManager
		// Используется для передачи событий во фронтенд.
		// Если планируется использовать только API без фронтенда - broadcaster можно убрать из зависимостей AppHandler.
		// Инициализировав broadcaster в main далее он используется в status_worker.
		broadcaster = broadcast.NewR3labsSSEAdapter(
			broadcast.MakeJWTTopicResolver(srvConfig.JWTSecretKey),
		)
	} else {
		broadcaster = broadcast.NewNoopAdapter(func(r *http.Request) (string, error) { return "noop", nil })
	}

	// создаём handlersContainer — контейнер зависимостей для всех хендлеров,
	// передаём в него хранилище, JWT ключ и SSE адаптер
	handlersContainer := api.NewHandlersContainer(storage, srvConfig, broadcaster)

	// запуск HTTP-сервера,
	// передаём готовый handlersContainer, содержащий все зависимости

	// создаем сервер и запускаем его
	srv, serverErrorCh := server.RunServer(srvConfig.RunAddress, handlersContainer)

	// запускаем воркер в отдельной горутине,
	// воркер периодически опрашивает БД и публикует обновления статусов служб через SSE
	workerCtx, workerCtxCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	var interval time.Duration = 5 * time.Second

	// если работаем с web-интерфейсом - запускаем status worker
	if srvConfig.WebInterface {
		// запуск status worker

		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.StatusWorker(workerCtx, storage, broadcaster, interval)
		}()
	}

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

	// если работаем с web-интерфейсом - ждем завершения worker с таймаутом
	if srvConfig.WebInterface {
		// останавливаем status worker (если был запущен)
		logger.Log.Info("Остановка status worker...")
		workerCtxCancel()

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
