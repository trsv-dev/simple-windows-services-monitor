package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

// TestServerStatusWorker Проверяет работу фонового воркера получения статусов серверов.
func TestServerStatusWorker(t *testing.T) {
	tests := []struct {
		name             string
		interval         time.Duration
		setupMock        func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage)
		contextDuration  time.Duration
		wantCacheCalls   int
		wantStorageCalls int
		wantSetStatuses  []models.ServerStatus
	}{
		{
			name:     "Успешно получены статусы двух серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
					{ServerID: 2, Address: "192.168.0.2"},
				}
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					Times(1)

				c.EXPECT().
					Set(gomock.Any()).
					Times(2)
			},
			contextDuration:  150 * time.Millisecond,
			wantCacheCalls:   2,
			wantStorageCalls: 1,
		},
		{
			name:     "Ошибка получения списка серверов - воркер продолжает работу",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				// первый вызов вернёт ошибку
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(nil, errors.New("database error")).
					Times(1)

				// второй и последующие вызовы будут успешны
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return([]*models.ServerStatus{
						{ServerID: 1, Address: "192.168.0.1"},
					}, nil).
					AnyTimes() // может быть много вызовов

				// минимум один сервер из успешных вызовов
				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(1) // минимум 1, может быть больше
			},
			contextDuration:  3 * time.Second,
			wantCacheCalls:   1,
			wantStorageCalls: 2,
		},
		{
			name:     "Пустой список серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return([]*models.ServerStatus{}, nil).
					Times(1)

				// если нет серверов, Set не вызывается
				c.EXPECT().
					Set(gomock.Any()).
					Times(0)
			},
			contextDuration:  150 * time.Millisecond,
			wantCacheCalls:   0,
			wantStorageCalls: 1,
		},
		{
			name:     "Контекст отменен - воркер завершается",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				// ни один вызов ListServersAddresses не должен произойти
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Times(0)

				c.EXPECT().
					Set(gomock.Any()).
					Times(0)
			},
			contextDuration:  0, // контекст отменяется сразу
			wantCacheCalls:   0,
			wantStorageCalls: 0,
		},
		{
			name:     "Несколько итераций - воркер получает статусы периодически",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
				}

				// ожидаем минимум 2 вызова за время работы
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					MinTimes(2)

				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(2)
			},
			contextDuration:  250 * time.Millisecond,
			wantCacheCalls:   2,
			wantStorageCalls: 2,
		},
		{
			name:     "Большое количество серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				// создаём 10 серверов
				var servers []*models.ServerStatus
				for i := 1; i <= 10; i++ {
					servers = append(servers, &models.ServerStatus{
						ServerID: int64(i),
						Address:  "192.168.0." + fmt.Sprint(i),
					})
				}

				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					Times(1)

				// ожидаем 10 вызовов Set (по одному на сервер)
				c.EXPECT().
					Set(gomock.Any()).
					Times(10)
			},
			contextDuration:  150 * time.Millisecond,
			wantCacheCalls:   10,
			wantStorageCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockWorkerStorage(ctrl)
			mockCache := mocks.NewMockStatusCacheStorage(ctrl)

			tt.setupMock(mockStorage, mockCache)

			// создаём контекст, который будет отменен через tt.contextDuration
			var ctx context.Context
			var cancel context.CancelFunc

			if tt.contextDuration == 0 {
				ctx, cancel = context.WithCancel(context.Background())
				cancel() // отменяем сразу
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), tt.contextDuration)
			}
			defer cancel()

			// запускаем воркер в отдельной горутине
			go ServerStatusWorker(ctx, mockStorage, mockCache, "5985", tt.interval)

			// ждём завершения воркера
			<-ctx.Done()
			// даём время на завершение горутины
			time.Sleep(50 * time.Millisecond)

			// проверяем, что GoMock ожидания были выполнены
			// (это происходит автоматически при defer ctrl.Finish())
		})
	}
}

// TestServerStatusWorker_SetStatuses Проверяет корректность статусов, которые попадают в кеш.
func TestServerStatusWorker_SetStatuses(t *testing.T) {
	tests := []struct {
		name            string
		setupMock       func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage)
		contextDuration time.Duration
	}{
		{
			name: "Статус содержит правильный ServerID и Address",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				servers := []*models.ServerStatus{
					{ServerID: 123, Address: "10.0.0.5"},
				}
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					Times(1)

				c.EXPECT().
					Set(gomock.Any()).
					Do(func(status models.ServerStatus) {
						// проверяем правильный ServerID и Address
						assert.Equal(t, int64(123), status.ServerID)
						assert.Equal(t, "10.0.0.5", status.Address)
					}).
					Times(1)
			},
			contextDuration: 150 * time.Millisecond,
		},
		{
			name: "Статус содержит поле Status (OK или Unreachable)",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
				}
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					Times(1)

				c.EXPECT().
					Set(gomock.Any()).
					Do(func(status models.ServerStatus) {
						// проверяем Status, а не ServerID
						assert.True(t, status.Status == "OK" || status.Status == "Unreachable")
					}).
					Times(1)
			},
			contextDuration: 150 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockWorkerStorage(ctrl)
			mockCache := mocks.NewMockStatusCacheStorage(ctrl)

			tt.setupMock(mockStorage, mockCache)

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextDuration)
			defer cancel()

			go ServerStatusWorker(ctx, mockStorage, mockCache, "5985", 100*time.Millisecond)

			<-ctx.Done()
			time.Sleep(50 * time.Millisecond)
		})
	}
}

// TestServerStatusWorker_RetryBehavior Проверяет поведение при ошибках и retry логику.
func TestServerStatusWorker_RetryBehavior(t *testing.T) {
	tests := []struct {
		name             string
		setupMock        func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage)
		contextDuration  time.Duration
		wantMinCallCount int
	}{
		{
			name: "После ошибки воркер продолжает попытки получить список серверов",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				gomock.InOrder(
					// первая попытка - ошибка (ровно 1 раз)
					s.EXPECT().
						ListServersAddresses(gomock.Any()).
						Return(nil, errors.New("connection timeout")).
						Times(1),

					// последующие попытки после retry - успех (может быть много)
					s.EXPECT().
						ListServersAddresses(gomock.Any()).
						Return([]*models.ServerStatus{
							{ServerID: 1, Address: "192.168.0.1"},
						}, nil).
						AnyTimes(), // может быть много вызовов
				)

				// минимум один сервер из успешных вызовов
				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(1) // может быть много вызовов
			},
			contextDuration:  3 * time.Second,
			wantMinCallCount: 2,
		},
		{
			name: "Успешный вызов сбрасывает интервал retry",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
				}

				// несколько успешных вызовов подряд
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					MinTimes(2)

				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(2)
			},
			contextDuration:  300 * time.Millisecond,
			wantMinCallCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockWorkerStorage(ctrl)
			mockCache := mocks.NewMockStatusCacheStorage(ctrl)

			tt.setupMock(mockStorage, mockCache)

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextDuration)
			defer cancel()

			go ServerStatusWorker(ctx, mockStorage, mockCache, "5985", 100*time.Millisecond)

			<-ctx.Done()
			time.Sleep(100 * time.Millisecond)
		})
	}
}
