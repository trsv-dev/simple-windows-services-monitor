package middleware

import (
	"context"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// LoginToContextMiddleware Middleware, который извлекает логин пользователя из JWT-токена,
// проверяет его и добавляет логин в контекст запроса.
// Это позволяет в дальнейшем получить логин из контекста (request.Context) в других обработчиках.
func LoginToContextMiddleware(JWTSecretKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenCookie, err := r.Cookie("JWT")
			if err != nil {
				// если cookie не найдена — считаем, что пользователь не аутентифицирован
				logger.Log.Error("Пользователь не аутентифицирован", logger.String("err", err.Error()))
				response.ErrorJSON(w, http.StatusUnauthorized, "Пользователь не аутентифицирован")
				return
			}

			// берем только саму строку токена, без префикса `JWT=`
			tokenString := tokenCookie.Value

			login, err := auth.GetLogin(tokenString, JWTSecretKey)
			if err != nil {
				// если не удалось извлечь логин — ошибка сервера
				logger.Log.Error("Ошибка идентификации пользователя", logger.String("err", err.Error()))
				response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка идентификации пользователя")
				return
			}

			// добавляем login в контекст запроса под ключом `contextkeys.Login`
			ctx := context.WithValue(r.Context(), contextkeys.Login, login)
			r = r.WithContext(ctx)

			// передаём управление следующему обработчику, уже с модифицированным запросом
			next.ServeHTTP(w, r)
		})
	}
}

//func LoginToContextMiddleware(h http.Handler) http.Handler {
//	f := func(w http.ResponseWriter, r *http.Request) {
//		token, err := r.Cookie("JWT")
//		if err != nil {
//			// если cookie не найдена — считаем, что пользователь не аутентифицирован
//			logger.Log.Error("Пользователь не аутентифицирован", logger.String("err", err.Error()))
//			response.ErrorJSON(w, http.StatusUnauthorized, "Пользователь не аутентифицирован")
//			return
//		}
//
//		// берем только саму строку токена, без префикса `JWT=`
//		tokenString := token.Value
//
//		// получаем логин пользователя
//		login, err := auth.GetLogin(tokenString)
//		if err != nil {
//			// если не удалось извлечь логин — ошибка сервера
//			logger.Log.Error("Ошибка идентификации пользователя", logger.String("err", err.Error()))
//			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка идентификации пользователя")
//			return
//		}
//
//		// встраиваем логин в контекст запроса под ключом `contextkeys.Login`
//		r = r.WithContext(context.WithValue(r.Context(), contextkeys.Login, login))
//		// передаём управление следующему обработчику, уже с модифицированным запросом
//		h.ServeHTTP(w, r)
//	}
//
//	return http.HandlerFunc(f)
//}
