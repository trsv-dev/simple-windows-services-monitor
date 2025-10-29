package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCorsMiddlewareAllowedOrigins Проверяет разрешённые origins.
func TestCorsMiddlewareAllowedOrigins(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	tests := []struct {
		name            string
		origin          string
		wantAllowOrigin string
		wantHeaderSet   bool
		wantNextCalled  bool
	}{
		{
			name:            "localhost:3000",
			origin:          "http://localhost:3000",
			wantAllowOrigin: "http://localhost:3000",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "127.0.0.1:3000",
			origin:          "http://127.0.0.1:3000",
			wantAllowOrigin: "http://127.0.0.1:3000",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "0.0.0.0:3000",
			origin:          "http://0.0.0.0:3000",
			wantAllowOrigin: "http://0.0.0.0:3000",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "null для file://",
			origin:          "null",
			wantAllowOrigin: "null",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "192.168.1.1",
			origin:          "http://192.168.1.1",
			wantAllowOrigin: "http://192.168.1.1",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "192.168.100.50",
			origin:          "http://192.168.100.50",
			wantAllowOrigin: "http://192.168.100.50",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "10.0.0.1",
			origin:          "http://10.0.0.1",
			wantAllowOrigin: "http://10.0.0.1",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "10.255.255.255",
			origin:          "http://10.255.255.255",
			wantAllowOrigin: "http://10.255.255.255",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "172.16.0.1",
			origin:          "http://172.16.0.1",
			wantAllowOrigin: "http://172.16.0.1",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
		{
			name:            "172.31.255.255",
			origin:          "http://172.31.255.255",
			wantAllowOrigin: "http://172.31.255.255",
			wantHeaderSet:   true,
			wantNextCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем что заголовок установлен правильно
			if tt.wantHeaderSet {
				assert.Equal(t, tt.wantAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))
			}

			// проверяем что next handler вызван
			assert.Equal(t, tt.wantNextCalled, nextCalled)
		})
	}
}

// TestCorsMiddlewareDisallowedOrigins Проверяет запрещённые origins.
func TestCorsMiddlewareDisallowedOrigins(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	tests := []struct {
		name            string
		origin          string
		wantAllowOrigin string // пустая строка = не установлен
		wantNextCalled  bool
	}{
		{
			name:            "запрещённый external origin",
			origin:          "http://example.com",
			wantAllowOrigin: "",
			wantNextCalled:  true,
		},
		{
			name:            "запрещённый https origin",
			origin:          "https://localhost:3000",
			wantAllowOrigin: "",
			wantNextCalled:  true,
		},
		{
			name:            "запрещённый другой port",
			origin:          "http://localhost:8080",
			wantAllowOrigin: "",
			wantNextCalled:  true,
		},
		{
			name:            "запрещённый внешний IP",
			origin:          "http://8.8.8.8",
			wantAllowOrigin: "",
			wantNextCalled:  true,
		},
		{
			name:            "запрещённый 11.x.x.x диапазон",
			origin:          "http://11.0.0.1",
			wantAllowOrigin: "",
			wantNextCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled = false

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем что заголовок НЕ установлен
			assert.Equal(t, tt.wantAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))

			// проверяем что next handler вызван
			assert.Equal(t, tt.wantNextCalled, nextCalled)
		})
	}
}

// TestCorsMiddlewareRequiredHeaders Проверяет обязательные CORS заголовки.
func TestCorsMiddlewareRequiredHeaders(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// обязательные заголовки
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "X-Requested-With")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "DELETE")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
	assert.Equal(t, "X-Is-Updated", w.Header().Get("Access-Control-Expose-Headers"))
}

// TestCorsMiddlewareOptionsRequest Проверяет обработку preflight OPTIONS запросов.
func TestCorsMiddlewareOptionsRequest(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodOptions, "/test", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// OPTIONS запрос должен вернуть 200 и НЕ вызвать next handler
	assert.Equal(t, http.StatusOK, w.Code)
	assert.False(t, nextCalled, "next handler НЕ должен быть вызван для OPTIONS")

	// но заголовки должны быть установлены
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// TestCorsMiddlewareNoOriginHeader Проверяет запрос без заголовка Origin.
func TestCorsMiddlewareNoOriginHeader(t *testing.T) {
	nextCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	// не устанавливаем Origin заголовок
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// Access-Control-Allow-Origin НЕ должен быть установлен
	assert.Equal(t, "", w.Header().Get("Access-Control-Allow-Origin"))

	// но другие CORS заголовки должны быть установлены
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))

	// next handler должен быть вызван
	assert.True(t, nextCalled)
}

// TestCorsMiddlewareAllHttpMethods Проверяет все HTTP методы.
func TestCorsMiddlewareAllHttpMethods(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodHead,
		http.MethodConnect,
		http.MethodTrace,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/test", nil)
			r.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// для OPTIONS - 200, для остальных - 200 (next handler)
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestCorsMiddlewareDifferentPorts Проверяет разные ports на localhost.
func TestCorsMiddlewareDifferentPorts(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	tests := []struct {
		name            string
		origin          string
		wantAllowOrigin string // пустая = не установлен
	}{
		{"localhost:3000", "http://localhost:3000", "http://localhost:3000"},
		{"localhost:8080", "http://localhost:8080", ""},
		{"localhost:5000", "http://localhost:5000", ""},
		{"localhost без port", "http://localhost", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

// TestCorsMiddlewareCredentialsAlwaysSet Проверяет что Credentials всегда установлены.
func TestCorsMiddlewareCredentialsAlwaysSet(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	origins := []string{
		"http://localhost:3000",
		"http://example.com", // даже для запрещённого origin
		"",                   // даже без origin
	}

	for _, origin := range origins {
		t.Run("origin: "+origin, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// credentials должны быть установлены ВСЕГДА
			assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

// TestCorsMiddlewarePrivateNetworkRanges Проверяет все приватные IP диапазоны.
func TestCorsMiddlewarePrivateNetworkRanges(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	tests := []struct {
		name   string
		origin string
	}{
		// 192.168.x.x
		{"192.168.0.1", "http://192.168.0.1"},
		{"192.168.1.254", "http://192.168.1.254"},
		{"192.168.255.255", "http://192.168.255.255"},

		// 10.x.x.x
		{"10.0.0.1", "http://10.0.0.1"},
		{"10.0.0.255", "http://10.0.0.255"},
		{"10.255.255.255", "http://10.255.255.255"},

		// 172.16-31.x.x
		{"172.16.0.1", "http://172.16.0.1"},
		{"172.20.0.1", "http://172.20.0.1"},
		{"172.31.255.255", "http://172.31.255.255"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Origin", tt.origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"),
				"приватный IP диапазон должен быть разрешён")
		})
	}
}

// TestCorsMiddlewareExposeHeaders Проверяет expose headers для пользовательских заголовков.
func TestCorsMiddlewareExposeHeaders(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Next handler может установить custom header
		w.Header().Set("X-Is-Updated", "true")
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что X-Is-Updated exposed
	assert.Equal(t, "X-Is-Updated", w.Header().Get("Access-Control-Expose-Headers"))

	// проверяем что сам заголовок установлен
	assert.Equal(t, "true", w.Header().Get("X-Is-Updated"))
}

// TestCorsMiddlewareConcurrentRequests Проверяет параллельные запросы.
func TestCorsMiddlewareConcurrentRequests(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CorsMiddleware(nextHandler)

	// запускаем 10 параллельных запросов
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
			done <- true
		}(i)
	}

	// ждём завершения
	for i := 0; i < 10; i++ {
		<-done
	}
}
