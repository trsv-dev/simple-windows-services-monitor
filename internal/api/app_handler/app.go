package app_handler

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
)

// AppHandler Структура для передачи общих зависимостей.
type AppHandler struct {
	Broadcaster  broadcast.Broadcaster
	AuthProvider auth.AuthProvider
}

// NewAppHandler Конструктор AppHandler.
func NewAppHandler(authProvider auth.AuthProvider, broadcaster broadcast.Broadcaster) *AppHandler {
	return &AppHandler{AuthProvider: authProvider, Broadcaster: broadcaster}
}
