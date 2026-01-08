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
			mockWinRMPort := "5985"
			tt.setupMock(mockStorage)

			handler := NewHealthHandler(mockStorage, mockStatusCache, mockChecker, mockWinRMPort)

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
	mockWinRMPort := "5985"

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

	handler := NewHealthHandler(mockStorage, mockCacheStorage, mockChecker, mockWinRMPort)

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
					Status:   "online",
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
				Status:   "online",
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
					Status:   "offline",
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
				Status:   "offline",
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
			mockWinRMPort := "5985"

			mockCtx := createContextWithCreds("test", int64(1), int64(1))

			tt.setupMock(mockStorage, mockCacheStorage)

			handler := NewHealthHandler(mockStorage, mockCacheStorage, mockChecker, mockWinRMPort)

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
