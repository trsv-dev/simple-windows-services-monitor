package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// NewServer Создание нового сервера.
func NewServer(runAddress string, router http.Handler) *http.Server {
	server := &http.Server{
		Addr:    runAddress,
		Handler: router,
	}

	return server
}

// RunServer Запуск сервера.
func RunServer(runAddress string, router http.Handler) error {
	server := NewServer(runAddress, router)

	serverError := make(chan error, 1)

	go func() {
		logger.Log.Info("Сервер запущен", logger.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Ошибка сервера", err)
			// останавливаем сервер
			serverError <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverError:
		//log.Printf("Ошибка сервера: %v", err)
		slog.Error("Ошибка сервера", err)
	case sig := <-stop:
		//log.Printf("Получен сигнал становки сервера: %v", sig)
		slog.Info("Получен сигнал становки сервера", sig)
	}

	//log.Println("Остановка сервера...")
	slog.Info("Остановка сервера...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		//log.Printf("Ошибка остановки сервера %v", err)
		slog.Error("Получен сигнал становки сервера", err)
		return fmt.Errorf("Ошибка остановки сервера %v\n", err)
	}

	//log.Println("Сервер остановлен")
	slog.Info("Сервер остановлен")
	return nil
}
