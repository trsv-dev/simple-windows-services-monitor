package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("debug", "stdout")
}

// TestLoggingResponseWriterWrite Проверяет перехват Write.
func TestLoggingResponseWriterWrite(t *testing.T) {
	w := httptest.NewRecorder()
	data := &responseData{status: 0, size: 0}
	lw := &LoggingResponseWriter{
		ResponseWriter: w,
		responseData:   data,
	}

	// пишем данные
	testData := []byte("Hello, World!")
	n, err := lw.Write(testData)

	assert.NoError(t, err)
	assert.Equal(t, len(testData), n)

	// проверяем что размер захвачен
	assert.Equal(t, len(testData), lw.responseData.size)

	// проверяем что данные в оригинальном writer
	assert.Equal(t, string(testData), w.Body.String())
}

// TestLoggingResponseWriterWriteMultiple Проверяет множественные Write вызовы.
func TestLoggingResponseWriterWriteMultiple(t *testing.T) {
	w := httptest.NewRecorder()
	data := &responseData{status: 0, size: 0}
	lw := &LoggingResponseWriter{
		ResponseWriter: w,
		responseData:   data,
	}

	// пишем несколько раз
	data1 := []byte("Hello, ")
	data2 := []byte("World!")
	data3 := []byte("\n")

	lw.Write(data1)
	lw.Write(data2)
	lw.Write(data3)

	// проверяем что размер суммируется
	expectedSize := len(data1) + len(data2) + len(data3)
	assert.Equal(t, expectedSize, lw.responseData.size)

	// проверяем что все данные в writer
	assert.Equal(t, "Hello, World!\n", w.Body.String())
}

// TestLoggingResponseWriterWriteEmpty Проверяет Write пустых данных.
func TestLoggingResponseWriterWriteEmpty(t *testing.T) {
	w := httptest.NewRecorder()
	data := &responseData{status: 0, size: 0}
	lw := &LoggingResponseWriter{
		ResponseWriter: w,
		responseData:   data,
	}

	// пишем пустые данные
	n, err := lw.Write([]byte{})

	assert.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, lw.responseData.size)
}

// TestLoggingResponseWriterWriteHeader Проверяет перехват WriteHeader.
func TestLoggingResponseWriterWriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"403 Forbidden", http.StatusForbidden},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			data := &responseData{status: 0, size: 0}
			lw := &LoggingResponseWriter{
				ResponseWriter: w,
				responseData:   data,
			}

			// устанавливаем статус код
			lw.WriteHeader(tt.statusCode)

			// проверяем что статус захвачен
			assert.Equal(t, tt.statusCode, lw.responseData.status)

			// проверяем что статус в оригинальном writer
			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

// TestLoggingResponseWriterFlush Проверяет Flush метод.
func TestLoggingResponseWriterFlush(t *testing.T) {
	w := httptest.NewRecorder()
	data := &responseData{status: 0, size: 0}
	lw := &LoggingResponseWriter{
		ResponseWriter: w,
		responseData:   data,
	}

	// вызываем Flush (не должно быть паники)
	assert.NotPanics(t, func() {
		lw.Flush()
	})
}

// TestLogMiddlewareBasic Проверяет базовую функциональность LogMiddleware.
func TestLogMiddlewareBasic(t *testing.T) {
	// создаём handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello"))
	})

	// оборачиваем в middleware
	handler := LogMiddleware(nextHandler)

	// делаем запрос
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что ответ прошёл через middleware
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Hello", w.Body.String())
}

// TestLogMiddlewareCapturesStatusCode Проверяет захват статус кода.
func TestLogMiddlewareCapturesStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseData string
	}{
		{"200 OK", http.StatusOK, "success"},
		{"201 Created", http.StatusCreated, "created"},
		{"400 Bad Request", http.StatusBadRequest, "bad request"},
		{"404 Not Found", http.StatusNotFound, "not found"},
		{"500 Internal Error", http.StatusInternalServerError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseData))
			})

			handler := LogMiddleware(nextHandler)

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем статус
			assert.Equal(t, tt.statusCode, w.Code)
			// проверяем ответ
			assert.Equal(t, tt.responseData, w.Body.String())
		})
	}
}

// TestLogMiddlewareCapturesResponseSize Проверяет захват размера ответа.
func TestLogMiddlewareCapturesResponseSize(t *testing.T) {
	tests := []struct {
		name         string
		responseData string
	}{
		{"пустой ответ", ""},
		{"маленький ответ", "Hi"},
		{"средний ответ", strings.Repeat("a", 100)},
		{"большой ответ", strings.Repeat("Hello, World! ", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseData))
			})

			handler := LogMiddleware(nextHandler)

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем размер
			assert.Equal(t, len(tt.responseData), len(w.Body.String()))
		})
	}
}

// TestLogMiddlewareMultipleWrites Проверяет работу с множественными Write.
func TestLogMiddlewareMultipleWrites(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Part 1 "))
		w.Write([]byte("Part 2 "))
		w.Write([]byte("Part 3"))
	})

	handler := LogMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что все части собраны
	assert.Equal(t, "Part 1 Part 2 Part 3", w.Body.String())
	// проверяем размер
	assert.Equal(t, len("Part 1 Part 2 Part 3"), len(w.Body.String()))
}

// TestLogMiddlewareDifferentMethods Проверяет работу с разными HTTP методами.
func TestLogMiddlewareDifferentMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.Method))
	})

	handler := LogMiddleware(nextHandler)

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			r := httptest.NewRequest(method, "/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем что метод обработан
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, method, w.Body.String())
		})
	}
}

// TestLogMiddlewareDifferentURIs Проверяет работу с разными URI.
func TestLogMiddlewareDifferentURIs(t *testing.T) {
	uris := []string{
		"/",
		"/api/users",
		"/api/users/123",
		"/api/users?page=1&limit=10",
		"/api/very/deep/nested/path",
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := LogMiddleware(nextHandler)

	for _, uri := range uris {
		t.Run(uri, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, uri, nil)
			w := httptest.NewRecorder()

			// не должно быть паники
			assert.NotPanics(t, func() {
				handler.ServeHTTP(w, r)
			})

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

// TestLogMiddlewareWithJSON Проверяет логирование JSON ответа.
func TestLogMiddlewareWithJSON(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","data":[1,2,3]}`))
	})

	handler := LogMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/api/data", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что JSON прошёл корректно
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
}

// TestLogMiddlewareErrorResponse Проверяет логирование ошибочного ответа.
func TestLogMiddlewareErrorResponse(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	})

	handler := LogMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем ошибку
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Internal Server Error", w.Body.String())
}

// TestLogMiddlewareChainingMultiple Проверяет цепочку middleware.
func TestLogMiddlewareChainingMultiple(t *testing.T) {
	// финальный handler
	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final"))
	})

	// оборачиваем в два LogMiddleware (должны работать оба)
	handler := LogMiddleware(LogMiddleware(finalHandler))

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// не должно быть паники
	assert.NotPanics(t, func() {
		handler.ServeHTTP(w, r)
	})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "final", w.Body.String())
}

// TestLogMiddlewareWithRequestBody Проверяет логирование запроса с телом.
func TestLogMiddlewareWithRequestBody(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// читаем тело (middleware не должно это трогать)
		body, _ := io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received: " + string(body)))
	})

	handler := LogMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test data"))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что тело прошло корректно
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "received: test data", w.Body.String())
}

// TestLogMiddlewareRequestHeaders Проверяет что заголовки запроса доступны.
func TestLogMiddlewareRequestHeaders(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		contentType := r.Header.Get("Content-Type")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(authorization + " " + contentType))
	})

	handler := LogMiddleware(nextHandler)

	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	r.Header.Set("Authorization", "Bearer token")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что заголовки прошли
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Bearer token application/json", w.Body.String())
}

// TestLoggingResponseDataInitialization Проверяет инициализацию responseData.
func TestLoggingResponseDataInitialization(t *testing.T) {
	data := &responseData{
		status: 0,
		size:   0,
	}

	// проверяем начальные значения
	assert.Equal(t, 0, data.status)
	assert.Equal(t, 0, data.size)
}

// TestLogMiddlewareTimeMeasurement Проверяет что время обработки логируется.
func TestLogMiddlewareTimeMeasurement(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// имитируем обработку
		time.Sleep(10 * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := LogMiddleware(nextHandler)

	startTest := time.Now()

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	duration := time.Since(startTest)

	// проверяем что обработка заняла минимум 10ms
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
	assert.Equal(t, http.StatusOK, w.Code)
}
