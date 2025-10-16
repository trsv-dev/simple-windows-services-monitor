package middleware

import (
	"net/http"
	"strings"
)

// CorsMiddleware - middleware для поддержки CORS с cookie аутентификацией
func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Разрешённые origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://0.0.0.0:3000",
			"null", // для file:// или direct access
		}

		isAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				isAllowed = true
				break
			}
		}

		// Разрешить весь диапазон локальной сети
		if !isAllowed && (strings.HasPrefix(origin, "http://192.168.") ||
			strings.HasPrefix(origin, "http://10.") ||
			strings.HasPrefix(origin, "http://172.")) {
			isAllowed = true
		}

		if isAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}

		// Обязательно для cookie аутентификации
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Разрешаем нужные заголовки
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, X-Requested-With")

		// Разрешаем HTTP методы
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")

		// Время кэширования preflight запросов
		w.Header().Set("Access-Control-Max-Age", "86400")

		// **Эта строка позволяет браузеру читать X-Is-Updated**
		w.Header().Set("Access-Control-Expose-Headers", "X-Is-Updated")

		// Обрабатываем preflight OPTIONS запросы
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
