package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

type Webhook struct {
	storage storage.Storage
}

func NewWebhook(storage storage.Storage) *Webhook {
	return &Webhook{
		storage: storage,
	}
}

// KeycloakEvent Структура события Keycloak
type KeycloakEvent struct {
	OperationType string `json:"operationType"` // "DELETE", "CREATE", "UPDATE"
	ResourceType  string `json:"resourceType"`  // "USER", "CLIENT", etc.
	ResourcePath  string `json:"resourcePath"`  // "users/xxx-xxx-xxx"
}

// HandleEvent Обработка событий из Keycloak.
func (wh *Webhook) HandleEvent(w http.ResponseWriter, r *http.Request) {
	var event KeycloakEvent

	// парсим тело запроса в структуру event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		logger.Log.Warn("Неверный формат запроса вебхука", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	// извлекаем ID пользователя
	userID := event.extractUserID()

	switch {
	// в дальнейшем сюда можно добавить диспетчеризацию для других поступающий из Keycloak событий

	// если приходит событие удаления пользователя - удаляем
	case event.OperationType == "DELETE" && event.ResourceType == "USER":
		if userDelErr := wh.handleUserDelete(r.Context(), userID); userDelErr != nil {
			var ErrUserIDNotFound *errs.ErrUserIDNotFound

			// если пользователя не существует
			if errors.As(userDelErr, &ErrUserIDNotFound) {
				logger.Log.Debug("Пользователя не существует", logger.String("err", userDelErr.Error()))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// если какая-то другая ошибка
			logger.Log.Error("Ошибка удаления пользователя", logger.String("userID", userID), logger.String("err", userDelErr.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Processing failed")
			return
		}

		logger.Log.Info("Пользователь удален", logger.String("userID", userID))
		w.WriteHeader(http.StatusNoContent)
		return

	// по умолчанию обрабатываем как неизвестное событие
	default:
		//logger.Log.Debug("Неизвестное событие", logger.String("op", event.OperationType), logger.String("res", event.ResourceType))
		//w.WriteHeader(http.StatusNoContent)
		return
	}
}

// Удаление пользователя из БД по событию в Keycloak.
func (wh *Webhook) handleUserDelete(ctx context.Context, userID string) error {
	deleteErr := wh.storage.DeleteUser(ctx, userID)
	if deleteErr != nil {
		return deleteErr
	}

	return nil
}

// Вспомогательная функция. Извлекает UUID пользователя из resourcePath.
func (e *KeycloakEvent) extractUserID() string {
	parts := strings.Split(e.ResourcePath, "/")

	if len(parts) > 0 {
		return parts[len(parts)-1] // всегда последний сегмент
	}

	return ""
}
