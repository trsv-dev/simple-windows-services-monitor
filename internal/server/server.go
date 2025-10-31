package server

import (
	"errors"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/router"
)

// NewServer Создание нового сервера.
func NewServer(runAddress string, handlers *api.HandlersContainer) *http.Server {
	mux := router.Router(handlers)

	server := &http.Server{
		Addr:    runAddress,
		Handler: mux,
	}

	return server
}

// RunServer Запускает сервер в горутине и возвращает сам сервер и канал ошибок.
func RunServer(runAddress string, handlers *api.HandlersContainer) (*http.Server, chan error) {
	server := NewServer(runAddress, handlers)

	// канал ошибок сервера
	serverErrorCh := make(chan error, 1)

	go func() {
		defer close(serverErrorCh)

		logger.Log.Info("Сервер запущен", logger.String("address", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("Ошибка сервера", logger.String("err", err.Error()))
			// отправляем ошибку в канал ошибок сервера
			serverErrorCh <- err
		}
	}()

	return server, serverErrorCh
}
