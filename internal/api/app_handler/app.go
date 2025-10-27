package app_handler

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
)

// AppHandler Структура для передачи общих зависимостей.
type AppHandler struct {
	Broadcaster  broadcast.Broadcaster
	TokenBuilder auth.TokenBuilder
	JWTSecretKey string
}

// NewAppHandler Конструктор AppHandler.
func NewAppHandler(JWTSecretKey string, tokenBuilder auth.TokenBuilder, broadcaster broadcast.Broadcaster) *AppHandler {
	return &AppHandler{JWTSecretKey: JWTSecretKey, TokenBuilder: tokenBuilder, Broadcaster: broadcaster}
}
