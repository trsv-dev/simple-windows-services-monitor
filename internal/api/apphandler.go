package api

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// AppHandler Структура для передачи общих зависимостей в хендлеры.
type AppHandler struct {
	storage      storage.Storage
	Broadcaster  broadcast.Broadcaster
	JWTSecretKey string
}

// NewAppHandler Конструктор AppHandler.
func NewAppHandler(storage storage.Storage, JWTSecretKey string, broadcaster broadcast.Broadcaster) *AppHandler {
	return &AppHandler{storage: storage, JWTSecretKey: JWTSecretKey, Broadcaster: broadcaster}
}
