package middleware

import (
	"context"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// UserLoginUserIdToContextMiddleware Middleware, который извлекает логин пользователя из JWT-токена,
// проверяет его и добавляет логин в контекст запроса.
// Это позволяет в дальнейшем получить логин из контекста (request.Context) в других обработчиках.
func UserLoginUserIdToContextMiddleware(JWTSecretKey string) func(http.Handler) http.Handler {
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

			// в claims будет лежать login и id
			claims, err := auth.GetClaims(tokenString, JWTSecretKey)
			if err != nil {
				// если не удалось извлечь логин — ошибка сервера
				logger.Log.Error("Ошибка идентификации пользователя", logger.String("err", err.Error()))
				response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка идентификации пользователя")
				return
			}

			// добавляем login и id в контекст запроса под ключом `contextkeys.UserID` и `contextkeys.ServerID` соответственно
			ctxWithLogin := context.WithValue(r.Context(), contextkeys.Login, claims.Login)
			ctxWithId := context.WithValue(ctxWithLogin, contextkeys.ID, claims.ID)
			r = r.WithContext(ctxWithId)

			// передаём управление следующему обработчику, уже с модифицированным запросом
			next.ServeHTTP(w, r)
		})
	}
}
