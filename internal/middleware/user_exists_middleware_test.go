package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

// TestUserExistsMiddleware Тестирует middleware проверки существования пользователя.
func TestUserExistsMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	// флаг и заглушка следующего обработчика
	var nextHandlerCalled bool
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	tests := []struct {
		name              string
		ctxUserID         interface{} // nil = ключ отсутствует в контексте
		setupMock         func()
		expectedStatus    int
		expectNextHandler bool
	}{
		{
			name:              "UserID отсутствует в контексте",
			ctxUserID:         nil,
			setupMock:         func() {},
			expectedStatus:    http.StatusInternalServerError,
			expectNextHandler: false,
		},
		{
			name:              "UserID пустая строка",
			ctxUserID:         "",
			setupMock:         func() {},
			expectedStatus:    http.StatusInternalServerError,
			expectNextHandler: false,
		},
		{
			name:              "UserID неверного типа",
			ctxUserID:         12345,
			setupMock:         func() {},
			expectedStatus:    http.StatusInternalServerError,
			expectNextHandler: false,
		},
		{
			name:      "Ошибка БД при проверке пользователя",
			ctxUserID: "user-123",
			setupMock: func() {
				mockStorage.EXPECT().
					UserExists(gomock.Any(), "user-123").
					Return(false, errors.New("database connection failed"))
			},
			expectedStatus:    http.StatusInternalServerError,
			expectNextHandler: false,
		},
		{
			name:      "Пользователь не найден в БД (рассинхрон)",
			ctxUserID: "user-404",
			setupMock: func() {
				mockStorage.EXPECT().
					UserExists(gomock.Any(), "user-404").
					Return(false, nil)
			},
			expectedStatus:    http.StatusForbidden,
			expectNextHandler: false,
		},
		{
			name:      "Пользователь существует в БД",
			ctxUserID: "user-valid",
			setupMock: func() {
				mockStorage.EXPECT().
					UserExists(gomock.Any(), "user-valid").
					Return(true, nil)
			},
			expectedStatus:    http.StatusOK,
			expectNextHandler: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandlerCalled = false // сброс флага перед каждым подтестом
			tt.setupMock()

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.ctxUserID != nil {
				ctx := context.WithValue(req.Context(), contextkeys.UserID, tt.ctxUserID)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handler := UserExistsMiddleware(mockStorage)(nextHandler)
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("ожидаемый статус %d, получен %d", tt.expectedStatus, rr.Code)
			}
			if nextHandlerCalled != tt.expectNextHandler {
				t.Errorf("ожидался вызов nextHandler: %v, получен: %v", tt.expectNextHandler, nextHandlerCalled)
			}
		})
	}
}
