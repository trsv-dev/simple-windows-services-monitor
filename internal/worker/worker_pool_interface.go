package worker

import (
	"context"
	"sync"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

//go:generate mockgen -destination=mocks/worker_pool_mock.go -package=mocks . WorkerPool

type WorkerPool interface {
	Start(ctx context.Context)
	Stop()
	Submit(serverStatus *models.ServerStatus) bool
}

type StatusWorkerPool struct {
	tasks      chan *models.ServerStatus
	workerFunc func(ctx context.Context, serverStatus *models.ServerStatus)
	poolSize   int
	wg         sync.WaitGroup
}

func NewStatusWorkerPool(poolSize int, workerFunc func(ctx context.Context, serverStatus *models.ServerStatus)) *StatusWorkerPool {
	return &StatusWorkerPool{
		tasks:      make(chan *models.ServerStatus, poolSize*20),
		poolSize:   poolSize,
		workerFunc: workerFunc,
	}
}

func (wp *StatusWorkerPool) Start(ctx context.Context) {
	for i := 0; i < wp.poolSize; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
}

func (wp *StatusWorkerPool) Stop() {
	close(wp.tasks)
	wp.wg.Wait()
}

func (wp *StatusWorkerPool) Submit(server *models.ServerStatus) bool {
	select {
	case wp.tasks <- server:
		return true
	default:
		// очередь переполнена, пропускаем задачу
		return false
	}
}

func (wp *StatusWorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	for {
		select {
		case <-ctx.Done():
			logger.Log.Debug("Завершение работы воркера по контексту", logger.Int("server status worker id=", id))
			return
		case server, ok := <-wp.tasks:
			if !ok {
				logger.Log.Debug("Канал tasks для StatusWorkerPool пуст. Завершение работы воркера", logger.Int("status_worker id", id))
				return
			}

			wp.workerFunc(ctx, server)
		}
	}
}
