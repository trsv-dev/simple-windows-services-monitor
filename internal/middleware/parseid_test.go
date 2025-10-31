package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestParseServerIDMiddlewareSuccess Проверяет успешное извлечение serverID из URL.
func TestParseServerIDMiddlewareSuccess(t *testing.T) {
	var capturedID int64
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		id := r.Context().Value(contextkeys.ServerID).(int64)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServerIDMiddleware(nextHandler)

	// создаём chi router для корректной работы URLParam
	router := chi.NewRouter()

	router.Get("/servers/{serverID}", handler.ServeHTTP)

	r := httptest.NewRequest(http.MethodGet, "/servers/123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// проверяем что next handler был вызван
	assert.True(t, nextCalled)

	// проверяем что ID правильно спарсился
	assert.Equal(t, int64(123), capturedID)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestParseServerIDMiddlewareMissingID Проверяет запрос без serverID в URL.
func TestParseServerIDMiddlewareMissingID(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServerIDMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/servers/", nil)
	w := httptest.NewRecorder()

	// вручную создаём chi RouteContext с пустым serverID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("serverID", "")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	// ВАЖНО: вызываем handler *напрямую*, не через router.ServeHTTP
	handler.ServeHTTP(w, r)

	assert.False(t, nextCalled)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "сервера")
}

// TestParseServerIDMiddlewareInvalidID Проверяет запрос с невалидным ID.
func TestParseServerIDMiddlewareInvalidID(t *testing.T) {
	tests := []struct {
		name     string
		serverID string
	}{
		{"строковый ID", "abc"},
		{"ID с буквами", "123abc"},
		{"ID с символами", "12@3"},
		{"ID равен нулю", "0"},
		{"ID c отрицательным числом", "-123"},
		{"отрицательное число со словом", "-123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := ParseServerIDMiddleware(nextHandler)

			// создаём chi router
			router := chi.NewRouter()

			router.Get("/servers/{serverID}", handler.ServeHTTP)

			r := httptest.NewRequest(http.MethodGet, "/servers/"+tt.serverID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			// проверяем что next handler НЕ был вызван
			assert.False(t, nextCalled, "для %s next handler НЕ должен быть вызван", tt.name)

			// проверяем статус
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestParseServerIDMiddlewareLargeID Проверяет работу с большим ID.
func TestParseServerIDMiddlewareLargeID(t *testing.T) {
	var capturedID int64

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(contextkeys.ServerID).(int64)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServerIDMiddleware(nextHandler)

	router := chi.NewRouter()

	router.Get("/servers/{serverID}", handler.ServeHTTP)

	// используем max int64
	r := httptest.NewRequest(http.MethodGet, "/servers/9223372036854775807", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// проверяем что большое число спарсилось
	assert.Equal(t, int64(9223372036854775807), capturedID)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestParseServiceIDMiddlewareSuccess Проверяет успешное извлечение serviceID из URL.
func TestParseServiceIDMiddlewareSuccess(t *testing.T) {
	var capturedID int64
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		id := r.Context().Value(contextkeys.ServiceID).(int64)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServiceIDMiddleware(nextHandler)

	// создаём chi router
	router := chi.NewRouter()

	router.Get("/services/{serviceID}", handler.ServeHTTP)

	r := httptest.NewRequest(http.MethodGet, "/services/456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// проверяем что next handler был вызван
	assert.True(t, nextCalled)

	// проверяем что ID правильно спарсился
	assert.Equal(t, int64(456), capturedID)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestParseServiceIDMiddlewareMissingID Проверяет запрос без serviceID в URL.
func TestParseServiceIDMiddlewareMissingID(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServiceIDMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/servers/", nil)
	w := httptest.NewRecorder()

	// вручную создаём chi RouteContext с пустым serviceID
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("serviceID", "")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	// ВАЖНО: вызываем handler *напрямую*, не через router.ServeHTTP
	handler.ServeHTTP(w, r)

	// проверяем что next handler НЕ был вызван
	assert.False(t, nextCalled)

	// проверяем статус
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// проверяем сообщение об ошибке
	assert.Contains(t, w.Body.String(), "службы")
}

// TestParseServiceIDMiddlewareInvalidID Проверяет запрос с невалидным ID.
func TestParseServiceIDMiddlewareInvalidID(t *testing.T) {
	tests := []struct {
		name      string
		serviceID string
	}{
		{"строковый ID", "xyz"},
		{"ID с буквами", "456abc"},
		{"ID с символами", "45@6"},
		{"ID равен нулю", "0"},
		{"ID c отрицательным числом", "-123"},
		{"отрицательное число со словом", "-123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := ParseServiceIDMiddleware(nextHandler)

			// создаём chi router
			router := chi.NewRouter()

			router.Get("/services/{serviceID}", handler.ServeHTTP)

			r := httptest.NewRequest(http.MethodGet, "/services/"+tt.serviceID, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			// проверяем что next handler НЕ был вызван
			assert.False(t, nextCalled)

			// проверяем статус
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestParseServerIDMiddlewareContextKey Проверяет что используется правильный ключ контекста.
func TestParseServerIDMiddlewareContextKey(t *testing.T) {
	var contextHasServerID bool
	var contextHasServiceID bool

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, contextHasServerID = r.Context().Value(contextkeys.ServerID).(int64)
		_, contextHasServiceID = r.Context().Value(contextkeys.ServiceID).(int64)
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServerIDMiddleware(nextHandler)

	router := chi.NewRouter()

	router.Get("/servers/{serverID}", handler.ServeHTTP)

	r := httptest.NewRequest(http.MethodGet, "/servers/123", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// проверяем что в контексте есть ServerID ключ
	assert.True(t, contextHasServerID)

	// проверяем что в контексте нет ServiceID ключа
	assert.False(t, contextHasServiceID)
}

// TestParseServiceIDMiddlewareContextKey Проверяет что используется правильный ключ контекста для serviceID.
func TestParseServiceIDMiddlewareContextKey(t *testing.T) {
	var contextHasServerID bool
	var contextHasServiceID bool

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, contextHasServerID = r.Context().Value(contextkeys.ServerID).(int64)
		_, contextHasServiceID = r.Context().Value(contextkeys.ServiceID).(int64)
		w.WriteHeader(http.StatusOK)
	})

	handler := ParseServiceIDMiddleware(nextHandler)

	router := chi.NewRouter()

	router.Get("/services/{serviceID}", handler.ServeHTTP)

	r := httptest.NewRequest(http.MethodGet, "/services/456", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	// проверяем что в контексте нет ServerID ключа
	assert.False(t, contextHasServerID)

	// проверяем что в контексте есть ServiceID ключ
	assert.True(t, contextHasServiceID)
}

// TestParseServerIDMiddlewareMultipleCalls Проверяет несколько вызовов с разными ID.
func TestParseServerIDMiddlewareMultipleCalls(t *testing.T) {
	testCases := []int64{1, 42, 999, 12345}

	for _, expectedID := range testCases {
		t.Run("ID: "+strconv.FormatInt(expectedID, 10), func(t *testing.T) {
			var capturedID int64

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := r.Context().Value(contextkeys.ServerID).(int64)
				capturedID = id
				w.WriteHeader(http.StatusOK)
			})

			handler := ParseServerIDMiddleware(nextHandler)

			router := chi.NewRouter()

			router.Get("/servers/{serverID}", handler.ServeHTTP)

			r := httptest.NewRequest(http.MethodGet, "/servers/"+strconv.FormatInt(expectedID, 10), nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			// проверяем правильный ID для каждого случая
			assert.Equal(t, expectedID, capturedID)
		})
	}
}
