package worker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestStatusWorkerPool_Start_Stop Проверяет корректный запуск и остановку пула воркеров.
func TestStatusWorkerPool_Start_Stop(t *testing.T) {
	tests := []struct {
		name     string
		poolSize int
	}{
		{
			name:     "Один воркер",
			poolSize: 1,
		},
		{
			name:     "Несколько воркеров",
			poolSize: 5,
		},
		{
			name:     "Много воркеров",
			poolSize: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processedCount := atomic.Int32{}

			workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
				processedCount.Add(1)
			}

			pool := NewStatusWorkerPool(tt.poolSize, workerFunc)
			ctx := context.Background()

			pool.Start(ctx)
			pool.Stop()

			// Убедитесь, что пул успешно запустился и остановился
			assert.NotNil(t, pool)
			assert.Equal(t, tt.poolSize, pool.poolSize)
		})
	}
}

// TestStatusWorkerPool_Submit_Success Проверяет успешную отправку задач в пул.
func TestStatusWorkerPool_Submit_Success(t *testing.T) {
	tests := []struct {
		name           string
		tasksCount     int
		poolSize       int
		expectedResult bool
	}{
		{
			name:           "Одна задача",
			tasksCount:     1,
			poolSize:       2,
			expectedResult: true,
		},
		{
			name:           "Много задач",
			tasksCount:     10,
			poolSize:       2,
			expectedResult: true,
		},
		{
			name:           "Задач меньше, чем воркеров",
			tasksCount:     2,
			poolSize:       5,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			processedCount := atomic.Int32{}
			wg := sync.WaitGroup{}
			wg.Add(tt.tasksCount)

			workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
				defer wg.Done()
				processedCount.Add(1)
				time.Sleep(10 * time.Millisecond) // имитация работы
			}

			pool := NewStatusWorkerPool(tt.poolSize, workerFunc)
			ctx := context.Background()

			pool.Start(ctx)

			// отправляем задачи
			submitResults := make([]bool, tt.tasksCount)
			for i := 0; i < tt.tasksCount; i++ {
				serverStatus := &models.ServerStatus{
					ServerID: int64(i),
					Address:  "192.168.0.1",
					Status:   "OK",
				}
				submitResults[i] = pool.Submit(serverStatus)
			}

			// проверяем, что все задачи были отправлены успешно
			for _, result := range submitResults {
				assert.Equal(t, tt.expectedResult, result)
			}

			// ждем обработки всех задач
			wg.Wait()

			pool.Stop()

			// проверяем, что все задачи были обработаны
			assert.Equal(t, int32(tt.tasksCount), processedCount.Load())
		})
	}
}

// TestStatusWorkerPool_Submit_QueueFull Проверяет поведение при переполнении очереди.
func TestStatusWorkerPool_Submit_QueueFull(t *testing.T) {
	blockCh := make(chan struct{})

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// блокируем воркер, чтобы очередь переполнилась
		<-blockCh
	}

	poolSize := 2
	pool := NewStatusWorkerPool(poolSize, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)

	// отправляем задачи, чтобы заполнить очередь (poolSize * 20 + несколько)
	maxQueueSize := poolSize * 20
	successCount := 0
	failCount := 0

	for i := 0; i < maxQueueSize+5; i++ {
		serverStatus := &models.ServerStatus{
			ServerID: int64(i),
			Address:  "192.168.0.1",
		}
		if pool.Submit(serverStatus) {
			successCount++
		} else {
			failCount++
		}
	}

	// все задачи должны быть в очереди
	assert.Equal(t, maxQueueSize, successCount)
	assert.Equal(t, 5, failCount) // лишние задачи не поместились

	close(blockCh)
	pool.Stop()
}

// TestStatusWorkerPool_ContextCancellation Проверяет завершение воркеров при отмене контекста.
func TestStatusWorkerPool_ContextCancellation(t *testing.T) {
	startedCount := atomic.Int32{}
	completedCount := atomic.Int32{}

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		startedCount.Add(1)

		// проверяем контекст ДО долгой операции
		select {
		case <-ctx.Done():
			return
		default:
		}

		// имитируем долгую операцию
		time.Sleep(500 * time.Millisecond)
		completedCount.Add(1)
	}

	pool := NewStatusWorkerPool(2, workerFunc)
	ctx, cancel := context.WithCancel(context.Background())

	pool.Start(ctx)

	// отправляем несколько задач
	for i := 0; i < 5; i++ {
		serverStatus := &models.ServerStatus{
			ServerID: int64(i),
			Address:  "192.168.0.1",
		}
		pool.Submit(serverStatus)
	}

	// даем время на обработку первых задач (но не всех!)
	time.Sleep(50 * time.Millisecond)

	// отменяем контекст - это прервет обработку текущих и оставшихся задач
	cancel()

	// даем время на завершение текущих операций
	time.Sleep(600 * time.Millisecond)

	pool.Stop()

	// проверяем результаты
	started := startedCount.Load()
	completed := completedCount.Load()

	// минимум одна задача должна была начаться
	assert.Greater(t, started, int32(0), "Хотя бы одна задача должна была начаться")

	// но завершиться должна было меньше, чем отправлено
	assert.Less(t, completed, int32(5), "Не все задачи должны были завершиться из-за отмены контекста")

	// завершённых должно быть меньше или равно начатым
	assert.LessOrEqual(t, completed, started, "Завершённых не может быть больше, чем начатых")
}

// TestStatusWorkerPool_ProcessingOrder Проверяет обработку задач в порядке отправки (FIFO).
func TestStatusWorkerPool_ProcessingOrder(t *testing.T) {
	var processedIDs []int64
	var mu sync.Mutex

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		mu.Lock()
		defer mu.Unlock()
		processedIDs = append(processedIDs, serverStatus.ServerID)
	}

	pool := NewStatusWorkerPool(1, workerFunc) // один воркер гарантирует FIFO порядок
	ctx := context.Background()

	pool.Start(ctx)

	// отправляем задачи с известными ID
	expectedIDs := []int64{1, 2, 3, 4, 5}
	for _, id := range expectedIDs {
		serverStatus := &models.ServerStatus{
			ServerID: id,
			Address:  "192.168.0.1",
		}
		pool.Submit(serverStatus)
	}

	pool.Stop()

	// проверяем, что задачи обработаны в правильном порядке
	assert.Equal(t, expectedIDs, processedIDs)
}

// TestStatusWorkerPool_ConcurrentSubmit Проверяет безопасность при конкурентной отправке задач.
func TestStatusWorkerPool_ConcurrentSubmit(t *testing.T) {
	processedCount := atomic.Int32{}

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		processedCount.Add(1)
	}

	pool := NewStatusWorkerPool(20, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)

	// конкурентно отправляем задачи из нескольких горутин
	goroutinesCount := 10
	tasksPerGoroutine := 100
	wg := sync.WaitGroup{}

	for g := 0; g < goroutinesCount; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < tasksPerGoroutine; i++ {
				serverStatus := &models.ServerStatus{
					ServerID: int64(goroutineID*tasksPerGoroutine + i),
					Address:  "192.168.0.1",
				}
				pool.Submit(serverStatus)
			}
		}(g)
	}

	wg.Wait()

	// даем время на обработку
	time.Sleep(100 * time.Millisecond)

	pool.Stop()

	// все задачи должны быть обработаны
	assert.Equal(t, int32(goroutinesCount*tasksPerGoroutine), processedCount.Load())
}

// TestStatusWorkerPool_EmptyPool Проверяет поведение пустого пула.
func TestStatusWorkerPool_EmptyPool(t *testing.T) {
	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// ничего не делаем
	}

	pool := NewStatusWorkerPool(2, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)
	// не отправляем ни одной задачи
	pool.Stop()

	// должно завершиться без ошибок
	assert.NotNil(t, pool)
}

// TestStatusWorkerPool_StopWithoutStart Проверяет остановку без запуска.
func TestStatusWorkerPool_StopWithoutStart(t *testing.T) {
	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// ничего не делаем
	}

	pool := NewStatusWorkerPool(2, workerFunc)

	// вызываем Stop без Start - должно быть безопасно
	pool.Stop()

	assert.NotNil(t, pool)
}

// TestStatusWorkerPool_SubmitAfterStop Проверяет отправку после остановки пула.
func TestStatusWorkerPool_SubmitAfterStop(t *testing.T) {
	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// ничего не делаем
	}

	pool := NewStatusWorkerPool(2, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)
	pool.Stop()

	// попытка отправить задачу после остановки
	serverStatus := &models.ServerStatus{
		ServerID: 1,
		Address:  "192.168.0.1",
	}

	// должно быть false, так как канал закрыт
	result := pool.Submit(serverStatus)
	assert.False(t, result)
}

// TestStatusWorkerPool_WorkerFuncError Проверяет обработку ошибок в workerFunc.
func TestStatusWorkerPool_WorkerFuncError(t *testing.T) {
	panicCalled := atomic.Bool{}

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// имитируем ошибку
		if serverStatus.ServerID == 2 {
			panicCalled.Store(true)
			// в реальности здесь может быть panic или error
		}
	}

	pool := NewStatusWorkerPool(2, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)

	// отправляем задачи, включая одну с ID=2
	for i := 1; i <= 5; i++ {
		serverStatus := &models.ServerStatus{
			ServerID: int64(i),
			Address:  "192.168.0.1",
		}
		pool.Submit(serverStatus)
	}

	pool.Stop()

	// проверяем, что обработка продолжилась несмотря на "ошибку"
	assert.True(t, panicCalled.Load())
}

// TestStatusWorkerPool_LargePayload Проверяет обработку больших объектов ServerStatus.
func TestStatusWorkerPool_LargePayload(t *testing.T) {
	processedCount := atomic.Int32{}

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// проверяем, что данные целые
		require.NotNil(t, serverStatus)
		require.Greater(t, len(serverStatus.Address), 0)
		processedCount.Add(1)
	}

	pool := NewStatusWorkerPool(2, workerFunc)
	ctx := context.Background()

	pool.Start(ctx)

	// отправляем задачи с большим адресом
	for i := 0; i < 10; i++ {
		serverStatus := &models.ServerStatus{
			ServerID: int64(i),
			Address:  "192.168." + string(rune(i)) + ".1 with very long address description",
			Status:   "OK",
		}
		pool.Submit(serverStatus)
	}

	pool.Stop()

	assert.Equal(t, int32(10), processedCount.Load())
}

// TestStatusWorkerPool_RapidStartStop Проверяет быстрый запуск и остановку.
func TestStatusWorkerPool_RapidStartStop(t *testing.T) {
	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// ничего не делаем
	}

	for i := 0; i < 10; i++ {
		pool := NewStatusWorkerPool(3, workerFunc)
		ctx := context.Background()

		pool.Start(ctx)
		pool.Stop()
	}

	// если мы здесь, то тест прошел успешно
	assert.True(t, true)
}

// TestStatusWorkerPool_ContextDeadline Проверяет поведение при deadline в контексте.
func TestStatusWorkerPool_ContextDeadline(t *testing.T) {
	processedCount := atomic.Int32{}

	workerFunc := func(ctx context.Context, serverStatus *models.ServerStatus) {
		// проверяем контекст
		select {
		case <-ctx.Done():
			return
		default:
			processedCount.Add(1)
			time.Sleep(50 * time.Millisecond)
		}
	}

	pool := NewStatusWorkerPool(2, workerFunc)

	// создаем контекст с коротким дедлайном
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	pool.Start(ctx)

	// отправляем задачи
	for i := 0; i < 10; i++ {
		serverStatus := &models.ServerStatus{
			ServerID: int64(i),
			Address:  "192.168.0.1",
		}
		pool.Submit(serverStatus)
	}

	// даем время на обработку
	time.Sleep(200 * time.Millisecond)

	pool.Stop()

	// некоторые задачи должны быть обработаны, но не все из-за дедлайна
	assert.Greater(t, processedCount.Load(), int32(0))
	assert.Less(t, processedCount.Load(), int32(10))
}
