package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// Структура для хранения данных ответа.
type responseData struct {
	status int
	size   int
}

// LoggingResponseWriter Структура, которой можно подменить оригинальный http.ResponseWriter
// для получения ответа и записи ответа в лог.
type LoggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

// Структура, которой можно подменить оригинальный http.ResponseWriter
// для получения ответа и записи ответа в лог.
func (l *LoggingResponseWriter) Write(b []byte) (int, error) {
	// записываем ответ, используя оригинальный http.ResponseWriter
	size, err := l.ResponseWriter.Write(b)
	// захватываем размер
	l.responseData.size += size

	return size, err
}

func (l *LoggingResponseWriter) WriteHeader(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWrite
	l.ResponseWriter.WriteHeader(statusCode)
	// захватываем код статуса
	l.responseData.status = statusCode
}

// LogMiddleware Middleware для логирования всех запросов.
func LogMiddleware(h http.Handler) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		data := responseData{
			status: 0,
			size:   0,
		}

		lw := LoggingResponseWriter{
			ResponseWriter: w,
			responseData:   &data,
		}

		start := time.Now()
		h.ServeHTTP(&lw, r)
		duration := time.Since(start)

		logger.Log.Debug("Got incoming HTTP request",
			logger.String("uri", r.RequestURI),
			logger.String("method", r.Method),
			logger.String("status", strconv.Itoa(data.status)),
			logger.String("duration", duration.String()),
			logger.String("size", strconv.Itoa(data.size)),
			//logger.String("remote_addr", r.RemoteAddr),
		)
	}

	return http.HandlerFunc(f)
}
