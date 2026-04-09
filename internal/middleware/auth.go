package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"

	//"github.com/trsv-dev/simple-windows-services-monitor/internal/auth/jwt"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// AuthMiddleware Middleware, который извлекает логин пользователя из токена,
// валидирует его и, если пользователь существует и токен валиден добавляет логин и UserID в контекст запроса.
// Если пользователя не существует - создает его в БД и добавляет логин и UserID в контекст запроса.
// Это позволяет в дальнейшем получить логин и UserID из контекста (request.Context) в других обработчиках.
func AuthMiddleware(storage storage.Storage, authProvider auth.AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string

			// извлекаем токен
			authHeader := r.Header.Get("Authorization")

			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				// если заголовка нет, пробуем куку JWT
				cookie, err := r.Cookie("JWT")
				if err == nil && cookie.Value != "" {
					token = cookie.Value
				}
			}
			if token == "" {
				// нет заголовка или неверный формат
				logger.Log.Error("Пользователь не аутентифицирован", logger.String("err", errors.New("хедер авторизации отсутствует или поврежден").Error()))
				response.ErrorJSON(w, http.StatusUnauthorized, "Пользователь не аутентифицирован")
				return
			}

			claims, err := authProvider.ValidateToken(r.Context(), token)
			if err != nil {
				// если не удалось извлечь логин — ошибка сервера
				logger.Log.Error("Ошибка идентификации пользователя", logger.String("err", err.Error()))
				//response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка идентификации пользователя")
				response.ErrorJSON(w, http.StatusUnauthorized, "Пользователь не аутентифицирован")
				return
			}

			claimUser := &models.User{ID: claims.ID, Login: claims.Login}

			// проверяем, существует ли пользователь в БД
			_, getErr := storage.GetUser(r.Context(), claimUser)

			// если пользователь не существует - создаем
			if getErr != nil {
				var ErrUserIDNotFound *errs.ErrUserIDNotFound

				switch {
				case errors.As(getErr, &ErrUserIDNotFound):
					createErr := storage.CreateUser(r.Context(), claimUser)
					if createErr != nil {
						var ErrLoginIsTaken *errs.ErrLoginIsTaken
						switch {
						// защита от гонки (создание пользователя параллельным запросом)
						case errors.As(createErr, &ErrLoginIsTaken):
							logger.Log.Info("Такой пользователь уже существует",
								logger.String("login", ErrLoginIsTaken.Login), logger.String("err", ErrLoginIsTaken.Err.Error()))
							response.ErrorJSON(w, http.StatusConflict, "Пользователь уже существует")
							return
						case createErr != nil:
							logger.Log.Error("Ошибка регистрации пользователя", logger.String("err", createErr.Error()))
							response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка регистрации пользователя")
							return
						}
					}

					logger.Log.Info("Пользователь зарегистрирован", logger.String("login", claimUser.Login), logger.String("ID", claimUser.ID))
					response.JSON(w, http.StatusOK, claimUser)
					return

				case getErr != nil:
					logger.Log.Error("Внутренняя ошибка сервера", logger.String("err", getErr.Error()))
					response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
					return
				}
			}

			// если пользователь уже существует, то добавляем login и UserID в контекст запроса под ключом
			// `contextkeys.Login` и `contextkeys.UserID` соответственно
			ctxWithLogin := context.WithValue(r.Context(), contextkeys.Login, claimUser.Login)
			ctxWithId := context.WithValue(ctxWithLogin, contextkeys.UserID, claimUser.ID)
			r = r.WithContext(ctxWithId)

			// передаём управление следующему обработчику, уже с модифицированным запросом
			next.ServeHTTP(w, r)
		})
	}
}
