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
	netutilsMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

// ============ Тесты базовой функциональности ServerStatusWorker ============

// TestServerStatusWorker Проверяет работу фонового воркера получения статусов серверов.
func TestServerStatusWorker(t *testing.T) {
	tests := []struct {
		name            string
		interval        time.Duration
		setupMock       func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker)
		contextDuration time.Duration
	}{
		{
			name:     "Успешно получены статусы двух серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
					{ServerID: 2, Address: "192.168.0.2"},
				}
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					Times(1)

				// оба сервера доступны
				ch.EXPECT().
					CheckWinRM(gomock.Any(), "192.168.0.1", "5985", time.Duration(0)).
					Return(true)
				ch.EXPECT().
					CheckICMP(gomock.Any(), "192.168.0.1", time.Duration(0)).
					Return(true)

				ch.EXPECT().
					CheckWinRM(gomock.Any(), "192.168.0.2", "5985", time.Duration(0)).
					Return(true)
				ch.EXPECT().
					CheckICMP(gomock.Any(), "192.168.0.2", time.Duration(0)).
					Return(true)

				c.EXPECT().
					Set(gomock.Any()).
					Times(2)
			},
			contextDuration: 150 * time.Millisecond,
		},
		{
			name:     "Ошибка получения списка серверов - воркер продолжает работу",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
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
					AnyTimes()

				ch.EXPECT().
					CheckWinRM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				ch.EXPECT().
					CheckICMP(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				// минимум один сервер из успешных вызовов
				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(1)
			},
			contextDuration: 3 * time.Second,
		},
		{
			name:     "Пустой список серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return([]*models.ServerStatus{}, nil).
					Times(1)

				// если нет серверов, Set не вызывается
				c.EXPECT().
					Set(gomock.Any()).
					Times(0)
			},
			contextDuration: 150 * time.Millisecond,
		},
		{
			name:     "Контекст отменен - воркер завершается",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
				// ни один вызов ListServersAddresses не должен произойти
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Times(0)

				c.EXPECT().
					Set(gomock.Any()).
					Times(0)
			},
			contextDuration: 0, // контекст отменяется сразу
		},
		{
			name:     "Несколько итераций - воркер получает статусы периодически",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
				}

				// ожидаем минимум 2 вызова за время работы
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					MinTimes(2)

				ch.EXPECT().
					CheckWinRM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				ch.EXPECT().
					CheckICMP(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(2)
			},
			contextDuration: 250 * time.Millisecond,
		},
		{
			name:     "Большое количество серверов",
			interval: 100 * time.Millisecond,
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
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

				ch.EXPECT().
					CheckWinRM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				ch.EXPECT().
					CheckICMP(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				// ожидаем 10 вызовов Set (по одному на сервер)
				c.EXPECT().
					Set(gomock.Any()).
					Times(10)
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
			mockChecker := netutilsMocks.NewMockChecker(ctrl)

			tt.setupMock(mockStorage, mockCache, mockChecker)

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
			poolSize := 100
			go ServerStatusWorker(ctx, mockStorage, mockCache, mockChecker, "5985", tt.interval, poolSize)

			// ждём завершения воркера
			<-ctx.Done()
			// даём время на завершение горутины
			time.Sleep(50 * time.Millisecond)

			// проверяем, что GoMock ожидания были выполнены
			// (это происходит автоматически при defer ctrl.Finish())
		})
	}
}

// ============ Тесты статусов серверов ============

func TestServerStatusWorker_AllStatuses(t *testing.T) {
	tests := []struct {
		name            string
		winrmOK         bool
		icmpOK          bool
		expectedStatus  models.Status
		contextDuration time.Duration
	}{
		{
			name:            "Статус OK - icmp и winrm работают",
			winrmOK:         true,
			icmpOK:          true,
			expectedStatus:  models.StatusOK,
			contextDuration: 150 * time.Millisecond,
		},
		{
			name:            "Статус Unreachable - icmp недоступен",
			winrmOK:         false, // winrm не должен быть вызван
			icmpOK:          false,
			expectedStatus:  models.StatusUnreachable,
			contextDuration: 150 * time.Millisecond,
		},
		{
			name:            "Статус Degraded - icmp доступен, winrm недоступен",
			winrmOK:         false,
			icmpOK:          true,
			expectedStatus:  models.StatusDegraded,
			contextDuration: 150 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockWorkerStorage(ctrl)
			mockCache := mocks.NewMockStatusCacheStorage(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)

			servers := []*models.ServerStatus{
				{ServerID: 1, Address: "192.168.0.1"},
			}

			mockStorage.EXPECT().
				ListServersAddresses(gomock.Any()).
				Return(servers, nil).
				Times(1)

			// Порядок вызовов: сначала ICMP
			mockChecker.EXPECT().
				CheckICMP(gomock.Any(), "192.168.0.1", time.Duration(0)).
				Return(tt.icmpOK)

			// WinRM дергаем только если icmpOK == true
			if tt.icmpOK {
				mockChecker.EXPECT().
					CheckWinRM(gomock.Any(), "192.168.0.1", "5985", time.Duration(0)).
					Return(tt.winrmOK)
			}

			mockCache.EXPECT().
				Set(gomock.Any()).
				Do(func(status models.ServerStatus) {
					assert.Equal(t, int64(1), status.ServerID)
					assert.Equal(t, "192.168.0.1", status.Address)
					assert.Equal(t, tt.expectedStatus, status.Status)
				}).
				Times(1)

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextDuration)
			defer cancel()

			poolSize := 100
			go ServerStatusWorker(ctx, mockStorage, mockCache, mockChecker, "5985", 100*time.Millisecond, poolSize)

			<-ctx.Done()
			time.Sleep(50 * time.Millisecond)
		})
	}
}

// ============ Тесты retry логики ============

// TestServerStatusWorker_RetryBehavior Проверяет поведение при ошибках и retry логику.
func TestServerStatusWorker_RetryBehavior(t *testing.T) {
	tests := []struct {
		name            string
		setupMock       func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker)
		contextDuration time.Duration
	}{
		{
			name: "После ошибки воркер продолжает попытки получить список серверов",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
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
						AnyTimes(),
				)

				ch.EXPECT().
					CheckWinRM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				ch.EXPECT().
					CheckICMP(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				// минимум один сервер из успешных вызовов
				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(1)
			},
			contextDuration: 3 * time.Second,
		},
		{
			name: "Успешный вызов сбрасывает интервал retry",
			setupMock: func(s *storageMocks.MockWorkerStorage, c *mocks.MockStatusCacheStorage, ch *netutilsMocks.MockChecker) {
				servers := []*models.ServerStatus{
					{ServerID: 1, Address: "192.168.0.1"},
				}

				// несколько успешных вызовов подряд
				s.EXPECT().
					ListServersAddresses(gomock.Any()).
					Return(servers, nil).
					MinTimes(2)

				ch.EXPECT().
					CheckWinRM(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				ch.EXPECT().
					CheckICMP(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true).
					AnyTimes()

				c.EXPECT().
					Set(gomock.Any()).
					MinTimes(2)
			},
			contextDuration: 300 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockWorkerStorage(ctrl)
			mockCache := mocks.NewMockStatusCacheStorage(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)

			tt.setupMock(mockStorage, mockCache, mockChecker)

			ctx, cancel := context.WithTimeout(context.Background(), tt.contextDuration)
			defer cancel()

			poolSize := 100
			go ServerStatusWorker(ctx, mockStorage, mockCache, mockChecker, "5985", 100*time.Millisecond, poolSize)

			<-ctx.Done()
			time.Sleep(100 * time.Millisecond)
		})
	}
}

// ============ Тесты checkServerStatus ============

// TestCheckServerStatus_AllCombinations Тестирует все комбинации доступности каналов.
func TestCheckServerStatus_AllCombinations(t *testing.T) {
	tests := []struct {
		name           string
		winrmOK        bool
		icmpOK         bool
		expectedStatus models.Status
	}{
		{"OK - icmp и winrm", true, true, "OK"},
		{"Unreachable - icmp=false, winrm не вызывается", false, false, "Unreachable"},
		{"Degraded - icmp=true, winrm=false", false, true, "Degraded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			checker := netutilsMocks.NewMockChecker(ctrl)
			cache := mocks.NewMockStatusCacheStorage(ctrl)

			srv := &models.ServerStatus{ServerID: 1, Address: "10.0.0.1"}

			// порядок вызовов сейчас: сначала ICMP, потом (опционально) WinRM
			checker.EXPECT().
				CheckICMP(gomock.Any(), "10.0.0.1", time.Duration(0)).
				Return(tt.icmpOK)

			if tt.icmpOK {
				checker.EXPECT().
					CheckWinRM(gomock.Any(), "10.0.0.1", "5985", time.Duration(0)).
					Return(tt.winrmOK)
			}

			cache.EXPECT().
				Set(gomock.AssignableToTypeOf(models.ServerStatus{})).
				Do(func(st models.ServerStatus) {
					assert.Equal(t, tt.expectedStatus, st.Status)
				})

			err := checkServerStatus(context.Background(), srv, cache, checker, "5985")
			assert.NoError(t, err)
		})
	}
}
