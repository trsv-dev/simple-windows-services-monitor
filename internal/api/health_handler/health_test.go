package health_handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	netutilsMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestHealthHandler_GetHealth Проверяет поведение эндпоинта /health.
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
			mockWinRMPort := "5985"
			tt.setupMock(mockStorage)

			handler := NewHealthHandler(mockStorage, mockChecker, mockWinRMPort)

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

	handler := NewHealthHandler(mockStorage, mockChecker, mockWinRMPort)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.GetHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}
