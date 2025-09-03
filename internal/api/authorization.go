package api

import (
	"encoding/json"
	"errors"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"

	"io"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// UserAuthorization Авторизация пользователей.
func (h *AppHandler) UserAuthorization(w http.ResponseWriter, r *http.Request) {
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
		response.ErrorJSON(w, http.StatusBadRequest, "Ошибка анмаршаллинга данных в модель User")
		return
	}

	if user.Login == "" || user.Password == "" {
		logger.Log.Error("Пустой логин или пароль")
		response.ErrorJSON(w, http.StatusBadRequest, "Пустой логин или пароль")
		return
	}

	verifiedUser, err := h.storage.GetUser(ctx, &user)
	var ErrWrongLoginOrPassword *errs.ErrWrongLoginOrPassword
	switch {
	case errors.As(err, &ErrWrongLoginOrPassword):
		logger.Log.Error("Неверная пара логин/пароль")
		response.ErrorJSON(w, http.StatusUnauthorized, "Неверная пара логин/пароль")
		return
	case err != nil:
		logger.Log.Error("Внутренняя ошибка сервера", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}

	tokenString, err := auth.BuildJWTToken(verifiedUser)
	if err != nil {
		logger.Log.Debug("Ошибка при создании JWT-токена", logger.String("jwt-token", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при создании JWT-токена")
		return
	}

	auth.CreateCookie(w, tokenString)

	logger.Log.Debug("Успешная авторизация пользователя", logger.String("login", verifiedUser.Login))
	response.SuccessJSON(w, http.StatusOK, "Пользователь авторизован")
}
