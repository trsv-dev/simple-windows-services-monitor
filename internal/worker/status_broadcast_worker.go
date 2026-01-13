package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// StatusBroadcastWorker Периодически "дергает" in-memory хранилище статусов серверов
// и публикует статусы серверов пользователей через Publisher.
func StatusBroadcastWorker(ctx context.Context, storage storage.Storage, statusCache health_storage.StatusCacheStorage, publisher broadcast.Broadcaster, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := publishServerStatuses(ctx, storage, statusCache, publisher); err != nil {
			logger.Log.Error("ошибка ServerStatusBroadcastWorker",
				logger.String("err", err.Error()))
		}

		select {
		case <-ctx.Done():
			logger.Log.Info("Завершение работы воркера StatusBroadcastWorker по контексту", logger.String("info", ctx.Err().Error()))
			return
		case <-ticker.C: // следующий цикл по таймеру
		}
	}
}

// Получает все текущие статусы серверов каждого пользователя из in-memory БД и публикует их через Publisher.
func publishServerStatuses(ctx context.Context, storage storage.Storage, statusCache health_storage.StatusCacheStorage, publisher broadcast.Broadcaster) error {
	users, err := storage.ListUsers(ctx)
	if err != nil {
		return err
	}

	for _, user := range users {
		statuses := statusCache.GetAllServerStatusesByUser(user.ID)

		b, err := json.Marshal(statuses)
		if err != nil {
			return err
		}

		topic := fmt.Sprintf("user-%d", user.ID)
		if err := publisher.Publish(topic, b); err != nil {
			return err
		}
	}

	return nil
}
