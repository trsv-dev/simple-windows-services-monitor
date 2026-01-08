package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// ServerStatusWorker Фоновый воркер, предназначенный для периодической
// проверки сетевой доступности зарегистрированных серверов.
//
// Воркер с заданным интервалом:
//   - получает список серверов (id и address) из хранилища,
//   - проверяет доступность каждого сервера по сети (WinRM порт),
//   - обновляет in-memory кэш статусов серверов.
//
// Воркер не изменяет состояние базы данных и не содержит бизнес-логики.
// Его задача — формирование и поддержание актуального состояния доступности
// серверов для использования другими компонентами приложения (HTTP-хендлерами,
// SSE-рассылкой и т.п.).
//
// Жизненный цикл воркера управляется через context.Context:
// при отмене контекста воркер корректно завершает работу.
//
// Взаимодействие с хранилищем осуществляется через узкий интерфейс WorkerStorage,
// что позволяет изолировать воркер от конкретной реализации хранилища
// и упростить тестирование.
func ServerStatusWorker(ctx context.Context, storage storage.WorkerStorage, statusCache health_storage.StatusCacheStorage, winrmPort string, interval time.Duration) {
	checker := netutils.NewNetworkChecker()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	retryAfter := 1 * time.Second
	maxRetry := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Log.Info("Завершение работы воркера ServerStatusWorker по контексту", logger.String("info", ctx.Err().Error()))
			return
		case <-ticker.C:
			servers, err := storage.ListServersAddresses(ctx)

			// если произошла ошибка получения списка серверов из БД - продолжаем "достукиваться" до БД
			// с увеличением временного интервала
			if err != nil {
				logger.Log.Warn("Список серверов недоступен из ServerStatusWorker", logger.String("err", err.Error()))

				select {
				case <-ctx.Done():
					return
				case <-time.After(retryAfter):
					retryAfter *= 2
					if retryAfter > maxRetry {
						retryAfter = maxRetry
					}
					continue
				}
			}

			// успешный вызов — сбрасываем интервал
			retryAfter = 1 * time.Second

			for _, server := range servers {
				// проверяем доступность сервера, если недоступен - возвращаем ошибку
				if !checker.IsHostReachable(ctx, server.Address, winrmPort, 0) {
					logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен", server.Address, server.ServerID))
					status := models.ServerStatus{ServerID: server.ServerID, Address: server.Address, Status: "Unreachable"}
					// если сервер недоступен, то кладем в кэш статусов модель со статусом "Unreachable"
					statusCache.Set(status)
					continue
				}

				// если сервер нормально отвечает, то кладем в кэш статусов модель со статусом "ОК"
				status := models.ServerStatus{ServerID: server.ServerID, Address: server.Address, Status: "OK"}
				statusCache.Set(status)
			}
		}
	}
}
