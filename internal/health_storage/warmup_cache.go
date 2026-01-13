package health_storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// WarmUpStatusCache "Прогрев" in-memory хранилища: загрузка существующих в БД серверов в in-memory кэш.
func WarmUpStatusCache(ctx context.Context, storage storage.WorkerStorage, statusCache StatusCacheStorage) error {
	servers, err := storage.ListServersAddresses(ctx)
	if err != nil {
		return err
	}

	for _, server := range servers {
		statusCache.Set(models.ServerStatus{
			ServerID: server.ServerID,
			UserID:   server.UserID,
			Address:  server.Address,
		})
	}

	return nil
}
