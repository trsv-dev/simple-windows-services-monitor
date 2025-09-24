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
)

// NewServer Создание нового сервера.
func NewServer(runAddress string, appHandler *api.AppHandler) *http.Server {
	mux := router.Router(appHandler)

	server := &http.Server{
		Addr:    runAddress,
		Handler: mux,
	}

	return server
}

// RunServer Запуск сервера.
func RunServer(runAddress string, appHandler *api.AppHandler) error {
	server := NewServer(runAddress, appHandler)

	// канал ошибок сервера
	serverError := make(chan error, 1)
	// канал системных сигналов
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Log.Info("Сервер запущен", logger.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
			// останавливаем сервер
			serverError <- err
		}
	}()

	// блокируемся тут в ожидании одного из вариантов завершения работы сервера
	select {
	case err := <-serverError:
		logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
	case sig := <-stop:
		logger.Log.Info("Получен сигнал остановки сервера", logger.String("sig", sig.String()))
	}

	logger.Log.Info("Остановка сервера...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := appHandler.Broadcaster.Close(); err != nil {
		logger.Log.Warn("Ошибка закрытия SSE адаптера", logger.String("err", err.Error()))
	}

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("Ошибка остановки сервера", logger.String("err", err.Error()))
		return fmt.Errorf("Ошибка остановки сервера %v\n", err)
	}

	logger.Log.Info("Сервер остановлен")
	return nil
}
