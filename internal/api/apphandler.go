package api

import "github.com/trsv-dev/simple-windows-services-monitor/internal/storage"

// AppHandler Структура для передачи общих зависимостей в хендлеры.
type AppHandler struct {
	storage storage.Storage
}

// NewAppHandler Конструктор AppHandler.
func NewAppHandler(storage storage.Storage) *AppHandler {
	return &AppHandler{storage: storage}
}
