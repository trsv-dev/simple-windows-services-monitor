package health_handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	statusCacheStorageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	netutilsMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// Создание контекста с данными о пользователе и сервере.
func createContextWithCreds(login string, userID, serverID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextkeys.Login, login)
	ctx = context.WithValue(ctx, contextkeys.ID, userID)
	ctx = context.WithValue(ctx, contextkeys.ServerID, serverID)
	return ctx
}

// TestHealthHandler_GetHealth Проверяет поведение эндпоинта /health_storage.
func TestHealthHandler_GetHealth(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(m *storageMocks.MockStorage)
		wantStatus  int
		wantMessage string
	}{
		{
			name: "БД доступна",
			setupMock: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					Ping(gomock.Any()).
					Return(nil)
			},
			wantStatus:  http.StatusOK,
			wantMessage: "OK",
		},
		{
			name: "БД недоступна",
			setupMock: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					Ping(gomock.Any()).
					Return(errors.New("connection failed"))
			},
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: "База данных недоступна\n", // http.Error добавляет \n
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)
			mockStatusCache := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
			tt.setupMock(mockStorage)

			handler := NewHealthHandler(mockStorage, mockStatusCache, mockChecker)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()

			handler.GetHealth(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, tt.wantMessage, string(body))
		})
	}
}

// TestHealthHandler_PingTimeout Проверяет таймаут контекста при Ping.
func TestHealthHandler_PingTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockCacheStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	mockStorage.EXPECT().
		Ping(gomock.Any()).
		DoAndReturn(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err() // вернем таймаут
			case <-time.After(3 * time.Second):
				return nil
			}
		})

	handler := NewHealthHandler(mockStorage, mockCacheStorage, mockChecker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.GetHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// TestHealthHandler_ServerStatus Проверяет получение статуса сервера.
func TestHealthHandler_ServerStatus(t *testing.T) {
	tests := []struct {
		name              string
		setupMock         func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage)
		wantStatus        int
		wantStatusContent models.ServerStatus
		wantErr           bool
		wantErrMessage    string
	}{
		{
			name: "Сервер найден с доступным статусом в кеше",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServer := models.Server{
					ID:      1,
					Address: "192.168.0.1",
				}
				mockStatus := models.ServerStatus{
					ServerID: 1,
					Address:  "192.168.0.1",
					Status:   "OK",
				}
				m.EXPECT().
					GetServer(gomock.Any(), int64(1), int64(1)).
					Return(&mockServer, nil).
					Times(1)

				c.EXPECT().
					Get(int64(1)).
					Return(mockStatus, true).
					Times(1)
			},
			wantStatus: http.StatusOK,
			wantStatusContent: models.ServerStatus{
				ServerID: 1,
				Address:  "192.168.0.1",
				Status:   "OK",
			},
			wantErr: false,
		},
		{
			name: "Сервер не найден в хранилище",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				errServerNotFound := errs.NewErrServerNotFound(int64(1), int64(1), errors.New("сервер не найден"))
				m.EXPECT().
					GetServer(gomock.Any(), int64(1), int64(1)).
					Return(nil, errServerNotFound).
					Times(1)
			},
			wantStatus:     http.StatusNotFound,
			wantErr:        true,
			wantErrMessage: "Сервер не найден",
		},
		{
			name: "Сервер найден, но статус не в кеше",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServer := models.Server{
					ID:      1,
					Address: "192.168.0.1",
				}
				m.EXPECT().
					GetServer(gomock.Any(), int64(1), int64(1)).
					Return(&mockServer, nil).
					Times(1)

				c.EXPECT().
					Get(int64(1)).
					Return(models.ServerStatus{}, false).
					Times(1)
			},
			wantStatus:     http.StatusInternalServerError,
			wantErr:        true,
			wantErrMessage: "Внутренняя ошибка сервера",
		},
		{
			name: "Ошибка при получении информации о сервере",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				m.EXPECT().
					GetServer(gomock.Any(), int64(1), int64(1)).
					Return(nil, errors.New("database connection error")).
					Times(1)
			},
			wantStatus:     http.StatusInternalServerError,
			wantErr:        true,
			wantErrMessage: "Ошибка при получении информации о сервере",
		},
		{
			name: "Сервер найден с разным статусом",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServer := models.Server{
					ID:      1,
					Address: "10.0.0.1",
				}
				mockStatus := models.ServerStatus{
					ServerID: 1,
					Address:  "10.0.0.1",
					Status:   "Unreachable",
				}
				m.EXPECT().
					GetServer(gomock.Any(), int64(1), int64(1)).
					Return(&mockServer, nil).
					Times(1)

				c.EXPECT().
					Get(int64(1)).
					Return(mockStatus, true).
					Times(1)
			},
			wantStatus: http.StatusOK,
			wantStatusContent: models.ServerStatus{
				ServerID: 1,
				Address:  "10.0.0.1",
				Status:   "Unreachable",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockCacheStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)

			mockCtx := createContextWithCreds("test", int64(1), int64(1))

			tt.setupMock(mockStorage, mockCacheStorage)

			handler := NewHealthHandler(mockStorage, mockCacheStorage, mockChecker)

			r := httptest.NewRequest("GET", "/servers/1/status", nil).WithContext(mockCtx)
			w := httptest.NewRecorder()

			handler.ServerStatus(w, r)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			if !tt.wantErr {
				// успешный случай - проверяем JSON
				var got models.ServerStatus
				err := json.Unmarshal(body, &got)
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatusContent, got)
			} else {
				// ошибка - проверяем текст ошибки
				bodyStr := string(body)
				assert.Contains(t, bodyStr, tt.wantErrMessage)
			}
		})
	}
}

// TestHealthHandler_ServersStatuses Проверяет получение массива статусов серверов.
func TestHealthHandler_ServersStatuses(t *testing.T) {
	tests := []struct {
		name              string
		setupMock         func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage)
		wantStatus        int
		wantStatusContent []models.ServerStatus
		wantErr           bool
		wantErrMessage    string
	}{
		{
			name: "Несколько серверов с статусами в кэше",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServers := []*models.Server{
					{
						ID:      1,
						Address: "192.168.0.1",
					},
					{
						ID:      2,
						Address: "192.168.0.2",
					},
				}
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(mockServers, nil).
					Times(1)

				mockStatuses := []models.ServerStatus{
					{
						ServerID: 1,
						Address:  "192.168.0.1",
						Status:   "OK",
					},
					{
						ServerID: 2,
						Address:  "192.168.0.2",
						Status:   "OK",
					},
				}

				gomock.InOrder(
					c.EXPECT().
						Get(int64(1)).
						Return(mockStatuses[0], true).
						Times(1),
					c.EXPECT().
						Get(int64(2)).
						Return(mockStatuses[1], true).
						Times(1),
				)
			},
			wantStatus: http.StatusOK,
			wantStatusContent: []models.ServerStatus{
				{
					ServerID: 1,
					Address:  "192.168.0.1",
					Status:   "OK",
				},
				{
					ServerID: 2,
					Address:  "192.168.0.2",
					Status:   "OK",
				},
			},
			wantErr: false,
		},
		{
			name: "Нет серверов у пользователя",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return([]*models.Server{}, nil).
					Times(1)
			},
			wantStatus:        http.StatusOK,
			wantStatusContent: []models.ServerStatus{},
			wantErr:           false,
		},
		{
			name: "Ошибка при получении списка серверов из БД",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(nil, errors.New("database connection error")).
					Times(1)
			},
			wantStatus:        http.StatusOK,
			wantStatusContent: []models.ServerStatus{},
			wantErr:           false,
		},
		{
			name: "Статус первого сервера не найден в кэше",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServers := []*models.Server{
					{
						ID:      1,
						Address: "192.168.0.1",
					},
					{
						ID:      2,
						Address: "192.168.0.2",
					},
				}
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(mockServers, nil).
					Times(1)

				mockStatus2 := models.ServerStatus{
					ServerID: 2,
					Address:  "192.168.0.2",
					Status:   "OK",
				}

				gomock.InOrder(
					c.EXPECT().
						Get(int64(1)).
						Return(models.ServerStatus{}, false).
						Times(1),
					c.EXPECT().
						Get(int64(2)).
						Return(mockStatus2, true).
						Times(1),
				)
			},
			wantStatus:     http.StatusInternalServerError,
			wantErr:        true,
			wantErrMessage: "Внутренняя ошибка сервера",
		},
		{
			name: "Все серверы с разными статусами",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServers := []*models.Server{
					{
						ID:      1,
						Address: "10.0.0.1",
					},
					{
						ID:      2,
						Address: "10.0.0.2",
					},
					{
						ID:      3,
						Address: "10.0.0.3",
					},
				}
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(mockServers, nil).
					Times(1)

				mockStatuses := []models.ServerStatus{
					{
						ServerID: 1,
						Address:  "10.0.0.1",
						Status:   "OK",
					},
					{
						ServerID: 2,
						Address:  "10.0.0.2",
						Status:   "Unreachable",
					},
					{
						ServerID: 3,
						Address:  "10.0.0.3",
						Status:   "Error",
					},
				}

				gomock.InOrder(
					c.EXPECT().
						Get(int64(1)).
						Return(mockStatuses[0], true).
						Times(1),
					c.EXPECT().
						Get(int64(2)).
						Return(mockStatuses[1], true).
						Times(1),
					c.EXPECT().
						Get(int64(3)).
						Return(mockStatuses[2], true).
						Times(1),
				)
			},
			wantStatus: http.StatusOK,
			wantStatusContent: []models.ServerStatus{
				{
					ServerID: 1,
					Address:  "10.0.0.1",
					Status:   "OK",
				},
				{
					ServerID: 2,
					Address:  "10.0.0.2",
					Status:   "Unreachable",
				},
				{
					ServerID: 3,
					Address:  "10.0.0.3",
					Status:   "Error",
				},
			},
			wantErr: false,
		},
		{
			name: "Статус среднего сервера не найден в кэше",
			setupMock: func(m *storageMocks.MockStorage, c *statusCacheStorageMocks.MockStatusCacheStorage) {
				mockServers := []*models.Server{
					{
						ID:      1,
						Address: "192.168.1.1",
					},
					{
						ID:      2,
						Address: "192.168.1.2",
					},
					{
						ID:      3,
						Address: "192.168.1.3",
					},
				}
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(mockServers, nil).
					Times(1)

				mockStatus1 := models.ServerStatus{
					ServerID: 1,
					Address:  "192.168.1.1",
					Status:   "OK",
				}

				gomock.InOrder(
					c.EXPECT().
						Get(int64(1)).
						Return(mockStatus1, true).
						Times(1),
					c.EXPECT().
						Get(int64(2)).
						Return(models.ServerStatus{}, false).
						Times(1),
					c.EXPECT().
						Get(int64(3)).
						Return(mockStatus1, true).
						Times(1),
				)
			},
			wantStatus:     http.StatusInternalServerError,
			wantErr:        true,
			wantErrMessage: "Внутренняя ошибка сервера",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockCacheStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)

			mockCtx := createContextWithCreds("test", int64(1), int64(0))

			tt.setupMock(mockStorage, mockCacheStorage)

			handler := NewHealthHandler(mockStorage, mockCacheStorage, mockChecker)

			r := httptest.NewRequest("GET", "/servers/statuses", nil).WithContext(mockCtx)
			w := httptest.NewRecorder()

			handler.ServersStatuses(w, r)

			resp := w.Result()
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)

			if !tt.wantErr {
				// успешный случай - проверяем JSON массив
				var got []models.ServerStatus
				err := json.Unmarshal(body, &got)
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatusContent, got)
			} else {
				// ошибка - проверяем текст ошибки
				bodyStr := string(body)
				assert.Contains(t, bodyStr, tt.wantErrMessage)
			}
		})
	}
}
