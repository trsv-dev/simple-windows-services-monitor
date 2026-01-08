package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

//go:generate mockgen -destination=mocks/worker_storage_mock.go -package=mocks . WorkerStorage

// WorkerStorage Описывает минимальный контракт хранилища,
// необходимый фоновым воркерам.
//
// Интерфейс намеренно вынесен отдельно от основного Storage,
// чтобы:
//   - не тянуть "толстый" Storage в воркеры
//   - избежать циклических зависимостей
//   - явно зафиксировать, какие операции разрешены воркерам
//
// Используется в ServerStatusWorker для получения списка серверов,
// которые необходимо периодически проверять.
type WorkerStorage interface {
	// ListServersAddresses Возвращает список серверов,
	// подлежащих периодической проверке доступности.
	//
	// Возвращаемый срез содержит минимальный набор данных
	// (ID и Address), достаточный для работы воркера.
	ListServersAddresses(ctx context.Context) ([]*models.ServerStatus, error)
}
