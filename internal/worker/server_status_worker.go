package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// Внутренний семафор пакета для ограничения числа одновременно работающих горутин с checkServerStatus.
type semaphore struct {
	semaCh chan struct{}
}

// NewSemaphore Возвращает новый семафор.
func newSemaphore(capacity int) *semaphore {
	return &semaphore{make(chan struct{}, capacity)}
}

// Acquire Отправляем пустую структуру в канал semaCh.
func (s *semaphore) Acquire() {
	s.semaCh <- struct{}{}
}

// Release Из канала semaCh убирается пустая структура.
func (s *semaphore) Release() {
	<-s.semaCh
}

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

	semaphore := newSemaphore(5)
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

			var wg sync.WaitGroup

			for _, server := range servers {
				wg.Add(1)
				s := server
				go func(srv *models.ServerStatus) {
					semaphore.Acquire()
					defer wg.Done()
					defer semaphore.Release()

					// проверяем доступность сервера и записываем статус
					checkServerStatus(ctx, srv, statusCache, checker, winrmPort)
				}(s)
			}

			wg.Wait()
		}
	}
}

func checkServerStatus(ctx context.Context, server *models.ServerStatus, statusCache health_storage.StatusCacheStorage, checker *netutils.NetworkChecker, winrmPort string) {
	var status string

	// вычисляем состояние один раз
	winrmOK := checker.CheckWinRM(ctx, server.Address, winrmPort, 0)
	icmpOK := checker.CheckICMP(ctx, server.Address, 0)

	switch {
	case winrmOK && icmpOK:
		status = "OK"
	case !winrmOK && !icmpOK:
		status = "Unreachable"
	default:
		// один из каналов (TCP или ICMP) не работает
		status = "Degraded"
	}

	if status != "OK" {
		logger.Log.Debug(fmt.Sprintf("Сервер %s, id=%d — %s (winrm=%v icmp=%v)", server.Address, server.ServerID, status, winrmOK, icmpOK))
	}

	logger.Log.Debug(fmt.Sprintf("Сервер %s, id=%d — %s (winrm=%v icmp=%v)", server.Address, server.ServerID, status, winrmOK, icmpOK))

	statusCache.Set(models.ServerStatus{ServerID: server.ServerID, Address: server.Address, Status: status})
}
