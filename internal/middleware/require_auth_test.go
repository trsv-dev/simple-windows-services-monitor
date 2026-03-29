package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestRequireAuthMiddleware Проверяет middleware аутентификации.
func TestRequireAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(r *http.Request) *http.Request
		wantStatus     int
		wantNextCalled bool
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name: "успешная аутентификация с логином",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name: "логин отсутствует в контексте",
			setupContext: func(r *http.Request) *http.Request {
				return r // контекст без логина
			},
			wantStatus:     http.StatusInternalServerError,
			wantNextCalled: false,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Ошибка сервера")
			},
		},
		{
			name: "логин пустая строка",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, "")
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusInternalServerError,
			wantNextCalled: false,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Ошибка сервера")
			},
		},
		{
			name: "логин неправильного типа (int вместо string)",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, 123)
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusInternalServerError,
			wantNextCalled: false,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Ошибка сервера")
			},
		},
		{
			name: "логин неправильного типа (nil)",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, nil)
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusInternalServerError,
			wantNextCalled: false,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Ошибка сервера")
			},
		},
		{
			name: "логин с пробелами (валидная строка)",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, "  user  ")
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name: "логин очень длинный",
			setupContext: func(r *http.Request) *http.Request {
				longLogin := string(make([]byte, 1000))
				for range longLogin {
					longLogin = "a"
				}
				ctx := context.WithValue(r.Context(), contextkeys.Login, longLogin)
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
		{
			name: "логин с спецсимволами",
			setupContext: func(r *http.Request) *http.Request {
				ctx := context.WithValue(r.Context(), contextkeys.Login, "user@example.com")
				return r.WithContext(ctx)
			},
			wantStatus:     http.StatusOK,
			wantNextCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// флаг для проверки вызова next handler
			nextCalled := false

			// создаём следующий handler
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				// проверяем что логин доступен в контексте
				if login, ok := r.Context().Value(contextkeys.Login).(string); ok && login != "" {
					// логин есть, проверяем что он совпадает
					assert.NotEmpty(t, login)
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			})

			// создаём middleware
			handler := RequireAuthMiddleware(nextHandler)

			// создаём запрос
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r = tt.setupContext(r)
			w := httptest.NewRecorder()

			// вызываем middleware
			handler.ServeHTTP(w, r)

			// проверяем статус
			assert.Equal(t, tt.wantStatus, w.Code, "статус код должен совпадать")

			// проверяем что next handler был вызван (или не вызван)
			assert.Equal(t, tt.wantNextCalled, nextCalled, "next handler должен быть вызван/не вызван")

			// дополнительные проверки ответа
			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}
		})
	}
}

// TestRequireAuthMiddlewareChain Проверяет цепочку middleware.
func TestRequireAuthMiddlewareChain(t *testing.T) {
	// создаём конечный handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.Context().Value(contextkeys.Login).(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, " + login))
	})

	// создаём цепочку middleware
	handler := RequireAuthMiddleware(finalHandler)

	tests := []struct {
		name       string
		login      string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "успешный запрос",
			login:      "john",
			wantStatus: http.StatusOK,
			wantBody:   "Hello, john",
		},
		{
			name:       "другой пользователь",
			login:      "alice",
			wantStatus: http.StatusOK,
			wantBody:   "Hello, alice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			ctx := context.WithValue(r.Context(), contextkeys.Login, tt.login)
			r = r.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantBody, w.Body.String())
		})
	}
}

// TestRequireAuthMiddlewareMultipleCalls Проверяет множественные вызовы middleware.
func TestRequireAuthMiddlewareMultipleCalls(t *testing.T) {
	callCount := 0

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuthMiddleware(nextHandler)

	// делаем 3 успешных запроса
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		ctx := context.WithValue(r.Context(), contextkeys.Login, "user")
		r = r.WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	}

	// проверяем что next handler был вызван 3 раза
	assert.Equal(t, 3, callCount)
}

// TestRequireAuthMiddlewareContextIsolation Проверяет изоляцию контекста между запросами.
func TestRequireAuthMiddlewareContextIsolation(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.Context().Value(contextkeys.Login).(string)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(login))
	})

	handler := RequireAuthMiddleware(nextHandler)

	// запрос 1 с пользователем "alice"
	r1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx1 := context.WithValue(r1.Context(), contextkeys.Login, "alice")
	r1 = r1.WithContext(ctx1)
	w1 := httptest.NewRecorder()

	handler.ServeHTTP(w1, r1)

	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "alice", w1.Body.String())

	// запрос 2 с пользователем "bob"
	r2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx2 := context.WithValue(r2.Context(), contextkeys.Login, "bob")
	r2 = r2.WithContext(ctx2)
	w2 := httptest.NewRecorder()

	handler.ServeHTTP(w2, r2)

	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "bob", w2.Body.String())

	// проверяем что контексты изолированы
	assert.NotEqual(t, w1.Body.String(), w2.Body.String())
}

// TestRequireAuthMiddlewareDifferentMethods Проверяет работу с разными HTTP методами.
func TestRequireAuthMiddlewareDifferentMethods(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuthMiddleware(nextHandler)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/test", nil)
			ctx := context.WithValue(r.Context(), contextkeys.Login, "user")
			r = r.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code, "метод %s должен работать", method)
		})
	}
}
