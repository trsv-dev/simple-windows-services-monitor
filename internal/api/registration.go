package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/utils"
)

// UserRegistration Регистрация пользователей.
func (h *AppHandler) UserRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	loginLen := 4
	passwordLen := 5

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Log.Error("Ошибка чтения тела запроса", logger.String("error", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Ошибка чтения тела запроса")
		return
	}

	var user models.User
	err = json.Unmarshal(data, &user)
	if err != nil {
		logger.Log.Error("Ошибка анмаршаллинга данных в модель User", logger.String("error", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка анмаршаллинга данных в модель User")
		return
	}

	switch {
	case len(user.Login) < loginLen || len(user.Password) < passwordLen:
		logger.Log.Error("Передан слишком короткий логин или пароль " +
			"(рекомендуется не менее 4 символов для логина и 5 символов для пароля)")
		response.ErrorJSON(w, http.StatusBadRequest, "Передан слишком короткий логин или пароль "+
			"(рекомендуется не менее 4 символов для логина и 5 символов для пароля)")
		return
	case !utils.IsAlphaNumeric(user.Login) || !utils.IsAlphaNumeric(user.Password):
		logger.Log.Error("Недопустимые символы в логине или пароле")
		response.ErrorJSON(w, http.StatusBadRequest, "Недопустимые символы в логине или пароле")
		return
	}

	err = h.storage.CreateUser(ctx, &user)
	var ErrLoginIsTaken *errs.ErrLoginIsTaken
	if err != nil {
		switch {
		case errors.As(err, &ErrLoginIsTaken):
			logger.Log.Info("Такой пользователь уже существует", logger.String("login", ErrLoginIsTaken.Login))
			response.ErrorJSON(w, http.StatusConflict, "Пользователь уже существует")
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	tokenString, err := auth.BuildJWTToken(&user)
	if err != nil {
		logger.Log.Debug("Ошибка при создании JWT-токена", logger.String("jwt-token", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при создании JWT-токена")
		return
	}

	auth.CreateCookie(w, tokenString)

	logger.Log.Debug("Успешная регистрация пользователя", logger.String("login", user.Login))
	response.SuccessJSON(w, http.StatusOK, "Пользователь зарегистрирован")
}
