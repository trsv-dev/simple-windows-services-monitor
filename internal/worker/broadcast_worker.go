package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// BroadcastWorker Периодически "дергает" БД и публикует статусы служб пользователей через Publisher.
func BroadcastWorker(ctx context.Context, storage storage.Storage, broadcaster broadcast.Broadcaster, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := fetchAndPublish(ctx, storage, broadcaster); err != nil {
			logger.Log.Error("ошибка воркера BroadcastWorker", logger.String("err", err.Error()))
		}

		select {
		case <-ctx.Done():
			logger.Log.Info("Завершение работы воркера BroadcastWorker по контексту", logger.String("info", ctx.Err().Error()))
			return
		case <-ticker.C: // следующий цикл по таймеру
		}
	}
}

// fetchAndPublish Получает статусы служб каждого пользователя из БД и публикует их через Publisher.
func fetchAndPublish(ctx context.Context, storage storage.Storage, publisher broadcast.Broadcaster) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	users, err := storage.ListUsers(fetchCtx)
	if err != nil {
		return err
	}

	for _, user := range users {
		statuses, err := storage.GetUserServiceStatuses(fetchCtx, user.ID)

		if err != nil {
			logger.Log.Error("ошибка получения статусов пользователя",
				logger.String("login", user.Login),
				logger.String("err", err.Error()))
			continue
		}

		b, err := json.Marshal(statuses)
		if err != nil {
			return err
		}

		// топик для конкретного пользователя создается в методе HTTPHandler()
		topic := fmt.Sprintf("user-%d", user.ID)
		if err = publisher.Publish(topic, b); err != nil {
			return err
		}
	}

	return nil
}
