package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/router"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// NewServer Создание нового сервера.
func NewServer(runAddress string, storage storage.Storage) *http.Server {
	appHandler := api.NewAppHandler(storage)
	mux := router.Router(appHandler)

	server := &http.Server{
		Addr:    runAddress,
		Handler: mux,
	}

	return server
}

// RunServer Запуск сервера.
func RunServer(runAddress string, storage storage.Storage) error {
	server := NewServer(runAddress, storage)

	serverError := make(chan error, 1)

	go func() {
		logger.Log.Info("Сервер запущен", logger.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
			// останавливаем сервер
			serverError <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverError:
		logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
	case sig := <-stop:
		logger.Log.Info("Получен сигнал остановки сервера", logger.String("sig", sig.String()))
	}

	logger.Log.Info("Остановка сервера...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Ошибка остановки сервера", logger.String("err", err.Error()))
		return fmt.Errorf("Ошибка остановки сервера %v\n", err)
	}

	logger.Log.Info("Сервер остановлен")
	return nil
}
