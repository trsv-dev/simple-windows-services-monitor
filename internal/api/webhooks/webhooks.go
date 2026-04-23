package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	models2 "github.com/trsv-dev/simple-windows-services-monitor/internal/auth/keycloak/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

type Webhook struct {
	storage storage.Storage
}

func NewWebhook(storage storage.Storage) *Webhook {
	return &Webhook{storage: storage}
}

// HandleEvent Автоопределение и обработка событий из Keycloak.
func (wh *Webhook) HandleEvent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Читаем тело для логирования и парсинга
	body, err := io.ReadAll(r.Body)

	if err != nil {
		logger.Log.Warn("Не удалось прочитать тело вебхука", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения запроса")
		return
	}

	// пробуем определить тип события по наличию поля "type"
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		logger.Log.Warn("Неверный формат JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат JSON")
		return
	}

	// определяем тип события. Если в "сырых" данных есть ключ "type" - это UserEvent,
	// если есть ключ "operationType" - это AdminEvent
	_, isUserEvent := raw["type"]
	_, isAdminEvent := raw["operationType"]

	switch {
	case isUserEvent:
		wh.handleUserEvent(w, r, body)
	case isAdminEvent:
		wh.handleAdminEvent(w, r, body)
	default:
		logger.Log.Debug("Неизвестный тип события (нет type или operationType)")
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

// Обработчик событий UserEvent.
func (wh *Webhook) handleUserEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	var event models2.KeycloakUserEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Log.Warn("Не удалось распарсить User Event", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения запроса")
		return
	}

	logger.Log.Debug("User Event",
		logger.String("type", event.Type),
		logger.String("userID", event.UserID),
		logger.String("ip", event.IPAddress))

	switch event.Type {
	// регистрация пользователя
	case "REGISTER":
		ID := event.UserID
		login, ok := event.Details["username"]
		if !ok {
			logger.Log.Error("username отсутствует в details")
			response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения запроса")
			return
		}

		// создание пользователя в БД
		if err := wh.storage.CreateUser(r.Context(), &models.User{ID: ID, Login: login}); err != nil {
			var userExistsErr *errs.ErrUserAlreadyExists

			if errors.As(err, &userExistsErr) {
				logger.Log.Info("Пользователь уже существует",
					logger.String("id", userExistsErr.ID), logger.String("err", userExistsErr.Err.Error()))
				response.ErrorJSON(w, http.StatusConflict, "Пользователь уже существует")
				return
			}

			logger.Log.Error("Ошибка регистрации пользователя", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка регистрации пользователя")
			return
		}

		logger.Log.Info("Пользователь зарегистрировался",
			logger.String("userID", event.UserID),
			logger.String("username", login),
			logger.String("ip", event.IPAddress))

	// вход в swsm
	case "LOGIN":
		logger.Log.Debug("Пользователь вошёл",
			logger.String("userID", event.UserID),
			logger.String("ip", event.IPAddress))

	// выход из swsm
	case "LOGOUT":
		logger.Log.Debug("Пользователь вышел",
			logger.String("userID", event.UserID))

	// получение токена пользователем
	case "CODE_TO_TOKEN":
		logger.Log.Debug("Получен токен",
			logger.String("userID", event.UserID),
			logger.String("clientID", event.ClientID))

	// получение refresh-токена
	//case "REFRESH_TOKEN":
	//	logger.Log.Debug("Получен refresh-токен",
	//		logger.String("userID", event.UserID),
	//		logger.String("clientID", event.ClientID))

	// ошибка регистрации
	case "REGISTER_ERROR":
		logger.Log.Warn("Ошибка регистрации",
			logger.String("error", event.Error),
			logger.String("ip", event.IPAddress))

	// ошибка входа
	case "LOGIN_ERROR":
		logger.Log.Debug("Ошибка входа",
			logger.String("error", event.Error),
			logger.String("userID", event.UserID))

	// Неизвестные user events
	default:
		//logger.Log.Debug("Неизвестный User Event",
		//	logger.String("type", event.Type),
		//	logger.String("userID", event.UserID))
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

// Обработчик событий AdminEvent.
func (wh *Webhook) handleAdminEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	var event models2.KeycloakAdminEvent
	if err := json.Unmarshal(body, &event); err != nil {
		logger.Log.Warn("Не удалось распарсить Admin Event", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения запроса")
		return
	}

	userID := event.ExtractUserID()

	switch {
	// удаление пользователя
	case event.OperationType == "DELETE" && event.ResourceType == "USER":
		if err := wh.handleUserDelete(r.Context(), userID); err != nil {
			var errNotFound *errs.ErrUserIDNotFound
			if errors.As(err, &errNotFound) {
				logger.Log.Debug("Пользователь уже удалён", logger.String("userID", userID))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			logger.Log.Error("Ошибка удаления пользователя",
				logger.String("userID", userID),
				logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка удаления пользователя")
			return
		}

		logger.Log.Info("Пользователь удалён (admin event)", logger.String("userID", userID))

	// создание пользователя (через админку Keycloak)
	case event.OperationType == "CREATE" && event.ResourceType == "USER":
		var userRep models2.UserRepresentation

		// парсим строку с данными о пользователе (user representation) в структуру
		// для дальнейшего получения данных о пользователе
		userRepErr := json.Unmarshal([]byte(event.Representation), &userRep)
		if userRepErr != nil {
			logger.Log.Warn("Не удалось распарсить User representation", logger.String("err", userRepErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения запроса")
			return
		}

		// создание пользователя в БД
		if err := wh.storage.CreateUser(r.Context(), &models.User{ID: userID, Login: userRep.Username}); err != nil {
			var userExistsErr *errs.ErrUserAlreadyExists

			if errors.As(err, &userExistsErr) {
				logger.Log.Info("Пользователь уже существует",
					logger.String("id", userExistsErr.ID), logger.String("err", userExistsErr.Err.Error()))
				response.ErrorJSON(w, http.StatusConflict, "Пользователь уже существует")
				return
			}

			logger.Log.Error("Ошибка регистрации пользователя", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка регистрации пользователя")
			return
		}

		logger.Log.Info("Пользователь создан (admin event)",
			logger.String("userID", userID))

	// обновление данных пользователя
	case event.OperationType == "UPDATE" && event.ResourceType == "USER":
		logger.Log.Debug("Данные пользователя обновлены",
			logger.String("userID", userID))

	// Неизвестные admin events
	default:
		logger.Log.Debug("Неизвестный Admin Event",
			logger.String("operation", event.OperationType),
			logger.String("resource", event.ResourceType))
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

// Вспомогательная функция. Удаление пользователя из БД по событию в Keycloak.
func (wh *Webhook) handleUserDelete(ctx context.Context, userID string) error {
	deleteErr := wh.storage.DeleteUser(ctx, userID)
	if deleteErr != nil {
		return deleteErr
	}

	return nil
}
