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
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/config"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/di_containers"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/server"
	storage "github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
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
	pgStorage, err := postgres.InitStorage(srvConfig.DatabaseURI, AESKeyBytes)
	if err != nil {
		logger.Log.Error("Не удалось инициировать хранилище (БД)", logger.String("err", err.Error()))
		os.Exit(1)
	}

	var handlersStorage storage.Storage = pgStorage
	var workersStorage storage.WorkerStorage = pgStorage

	tokenBuilder := auth.NewJWTTokenBuilder()
	var broadcaster broadcast.Broadcaster

	if srvConfig.WebInterface {
		// создание SSE Publisher/Subscriber,
		// используем r3labs/sse через адаптер, реализующий интерфейс SubscriberManager
		// Используется для передачи событий во фронтенд.
		// Если планируется использовать только API без фронтенда - broadcaster можно убрать из зависимостей AppHandler.
		// Инициализировав broadcaster в main далее он используется в ServiceBroadcastWorker.
		broadcaster = broadcast.NewR3labsSSEAdapter(
			broadcast.MakeJWTTopicResolver(srvConfig.JWTSecretKey, tokenBuilder),
		)
	} else {
		broadcaster = broadcast.NewNoopAdapter(func(r *http.Request) (string, error) { return "noop", nil })
	}

	// создаем in-memory хранилище для мониторинга статусов серверов
	statusCache := health_storage.NewStatusCache()

	// "прогрев" in-memory хранилища: загрузка существующих в БД серверов в in-memory кэш
	ctx, done := context.WithCancel(context.Background())
	defer done()

	if warmUpErr := health_storage.WarmUpStatusCache(ctx, workersStorage, statusCache); warmUpErr != nil {
		log.Fatal(warmUpErr.Error())
	}

	// создаем сетевой чекер
	netChecker := netutils.NewNetworkChecker()

	// создаём handlersContainer — контейнер зависимостей для всех хендлеров,
	// передаём в него хранилище, JWT ключ, SSE адаптер и инструмент проверки серверов по сети
	handlersContainer := di_containers.NewHandlersContainer(handlersStorage, statusCache, srvConfig, broadcaster, tokenBuilder, netChecker)

	// запуск HTTP-сервера,
	// передаём готовый handlersContainer, содержащий все зависимости

	// создаем сервер и запускаем его
	srv, serverErrorCh := server.RunServer(srvConfig.RunAddress, handlersContainer)

	// запускаем воркеры в отдельных горутинах:
	// - воркер worker.ServiceBroadcastWorker периодически опрашивает БД и публикует обновления статусов служб через SSE,
	// - воркер worker.ServerStatusWorker периодически достает из БД слайс всех серверов и получает их статус, сохраняя его в in-memory хранилище,
	// - воркер worker.StatusBroadcastWorker периодически "дергает" in-memory хранилище статусов серверов
	// и публикует статусы серверов пользователей через SSE
	workersCtx, workersCtxCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	var statusWorkerInterval time.Duration = 60 * time.Second
	poolSize := 100

	wg.Add(1)
	go func() {
		defer wg.Done()
		worker.ServerStatusWorker(workersCtx, workersStorage, statusCache, netChecker, srvConfig.WinRMPort, statusWorkerInterval, poolSize)
	}()

	// если работаем с web-интерфейсом - запускаем воркер ServiceBroadcastWorker для публикации статусов служб через SSE
	// и StatusBroadcastWorker для публикации статусов серверов через SSE
	if srvConfig.WebInterface {
		// запуск воркер ServiceBroadcastWorker
		var serviceBroadcastInterval time.Duration = 5 * time.Second

		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.ServiceBroadcastWorker(workersCtx, handlersStorage, broadcaster, serviceBroadcastInterval)
		}()

		// запуск воркер StatusBroadcastWorker
		statusBroadcastInterval := 2 * time.Second

		wg.Add(1)
		go func() {
			defer wg.Done()
			worker.StatusBroadcastWorker(workersCtx, handlersStorage, statusCache, broadcaster, statusBroadcastInterval)
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

	// останавливаем воркеры
	workersCtxCancel()

	// ждём завершения всех воркеров с таймаутом
	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
	}()

	select {
	case <-workersDone:
		logger.Log.Info("Воркеры остановлены")
	case <-time.After(5 * time.Second):
		logger.Log.Warn("Таймаут ожидания воркеров")
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
	if err = handlersStorage.Close(); err != nil {
		logger.Log.Error("Ошибка закрытия соединения с БД", logger.String("err", err.Error()))
	}
	logger.Log.Info("Успешное закрытие соединения с БД")

	logger.Log.Info("Приложение завершено")
}
