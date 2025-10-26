package authorization_handler

import (
	"encoding/json"
	"errors"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"

	"io"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// AuthorizationHandler Обработчик авторизации.
type AuthorizationHandler struct {
	storage      storage.Storage
	JWTSecretKey string
}

// NewAuthorizationHandler Конструктор AuthorizationHandler.
func NewAuthorizationHandler(storage storage.Storage, JWTSecretKey string) *AuthorizationHandler {
	return &AuthorizationHandler{
		storage:      storage,
		JWTSecretKey: JWTSecretKey,
	}
}

// UserAuthorization Авторизация пользователей.
func (h *AuthorizationHandler) UserAuthorization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Error("Ошибка чтения тела запроса", logger.String("error", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка чтения тела запроса")
		return
	}

	var user models.User
	err = json.Unmarshal(body, &user)
	if err != nil {
		logger.Log.Error("Ошибка анмаршаллинга данных в модель User", logger.String("error", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err := user.Validate(); err != nil {
		logger.Log.Error("Ошибка при валидации данных пользователя", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	verifiedUser, err := h.storage.GetUser(ctx, &user)
	var ErrWrongLoginOrPassword *errs.ErrWrongLoginOrPassword
	switch {
	case errors.As(err, &ErrWrongLoginOrPassword):
		logger.Log.Error("Неверная пара логин/пароль",
			logger.String("err", ErrWrongLoginOrPassword.Err.Error()))
		response.ErrorJSON(w, http.StatusUnauthorized, "Неверная пара логин/пароль")
		return
	case err != nil:
		logger.Log.Error("Внутренняя ошибка сервера", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}

	tokenString, err := auth.BuildJWTToken(verifiedUser, h.JWTSecretKey)
	if err != nil {
		logger.Log.Debug("Ошибка при создании JWT-токена", logger.String("jwt-token", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при создании JWT-токена")
		return
	}

	auth.CreateCookie(w, tokenString)

	logger.Log.Debug("Успешная авторизация пользователя", logger.String("login", verifiedUser.Login))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response.AuthResponse{
		Message: "Пользователь авторизован",
		Login:   user.Login,
		Token:   tokenString,
	})
	if err != nil {
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
