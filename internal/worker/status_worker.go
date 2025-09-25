package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// StatusWorker Периодически "дергает" БД и публикует статусы служб пользователей через Publisher.
func StatusWorker(ctx context.Context, storage storage.Storage, broadcaster broadcast.Broadcaster, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := fetchAndPublish(ctx, storage, broadcaster); err != nil {
			logger.Log.Error("ошибка воркера", logger.String("err", err.Error()))
		}

		select {
		case <-ctx.Done():
			logger.Log.Info("завершение работы воркера по контексту", logger.String("info", ctx.Err().Error()))
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
			//return err
		}

		b, err := json.Marshal(statuses)
		if err != nil {
			return err
		}

		// топик для конкретного пользователя создается в методе Publish
		if err = publisher.Publish(user.Login, b); err != nil {
			return err
		}
	}

	return nil
}
