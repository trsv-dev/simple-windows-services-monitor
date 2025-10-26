package app_handler

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
)

// AppHandler Структура для передачи общих зависимостей.
type AppHandler struct {
	Broadcaster  broadcast.Broadcaster
	JWTSecretKey string
}

// NewAppHandler Конструктор AppHandler.
func NewAppHandler(JWTSecretKey string, broadcaster broadcast.Broadcaster) *AppHandler {
	return &AppHandler{JWTSecretKey: JWTSecretKey, Broadcaster: broadcaster}
}
