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

// ServerStatusWorker Фоновый воркер с пулом, предназначенный для периодической
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
func ServerStatusWorker(ctx context.Context,
	storage storage.WorkerStorage,
	statusCache health_storage.StatusCacheStorage,
	netChecker netutils.Checker,
	winrmPort string,
	interval time.Duration,
	poolSize int,
) {
	// создаем пул воркеров
	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		if checkServerErr := checkServerStatus(ctx, serverStatus, statusCache, netChecker, winrmPort); checkServerErr != nil {
			// проверяем доступность сервера и записываем статус
			// если из checkServerStatus вернулась ошибка - пропускаем сервер
			logger.Log.Debug("Ошибка проверки статуса сервера",
				logger.Int64("server_id", serverStatus.ServerID),
				logger.String("address", serverStatus.Address),
				logger.String("error", checkServerErr.Error()),
			)
			return
		}
	}

	pool := NewStatusWorkerPool(poolSize, workerFunc)

	// запускаем пул воркеров
	pool.Start(ctx)
	defer pool.Stop()

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

			if len(servers) <= 0 {
				continue
			}

			// отправляем задачи в пул
			var skipped int
			for _, serverStatus := range servers {
				select {
				case <-ctx.Done():
					return
				default:
					if !pool.Submit(serverStatus) {
						skipped++
					}
				}
			}

			if skipped > 0 {
				total := len(servers)
				submitted := total - skipped
				logger.Log.Debug("Результат отправки задач",
					logger.Int("total", total),
					logger.Int("submitted", submitted),
					logger.Int("skipped", skipped))
			}
		}
	}
}

// Вычисление статуса сервера (CheckWinRM, CheckICMP) и запись модели статуса сервера в in-memory хранилище статусов.
func checkServerStatus(ctx context.Context, server *models.ServerStatus, statusCache health_storage.StatusCacheStorage, netChecker netutils.Checker, winrmPort string) error {
	// ограничиваем суммарное время проверки одного сервера
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var status string

	var winrmOK, icmpOK bool

	icmpOK = netChecker.CheckICMP(checkCtx, server.Address, 0)
	if !icmpOK {
		status = "Unreachable"
	} else {
		winrmOK = netChecker.CheckWinRM(checkCtx, server.Address, winrmPort, 0)
		if winrmOK {
			status = "OK"
		} else {
			status = "Degraded"
		}
	}

	if status != "OK" {
		logger.Log.Debug(fmt.Sprintf("Сервер %s, id=%d — %s (winrm=%v icmp=%v)", server.Address, server.ServerID, status, winrmOK, icmpOK))
	}

	serverStatus := models.ServerStatus{ServerID: server.ServerID, UserID: server.UserID, Address: server.Address, Status: status}

	if err := serverStatus.ValidateStatus(status); err != nil {
		return err
	}

	statusCache.Set(serverStatus)
	return nil
}
