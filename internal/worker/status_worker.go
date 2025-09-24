package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// StatusWorker Периодически "дергает" БД и публикует статусы служб через Publisher.
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

// fetchAndPublish Получает статусы из БД и публикует их через Publisher.
func fetchAndPublish(ctx context.Context, storage storage.Storage, publisher broadcast.Broadcaster) error {
	fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	statuses, err := storage.GetAllServiceStatuses(fetchCtx)
	if err != nil {
		return err
	}

	b, err := json.Marshal(statuses)
	if err != nil {
		return err
	}

	if err = publisher.Publish("services", b); err != nil {
		return err
	}

	return nil
}
