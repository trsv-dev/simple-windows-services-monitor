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
)

// UserRegistration Регистрация пользователей.
func (h *AppHandler) UserRegistration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

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
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err := user.Validate(); err != nil {
		logger.Log.Error("Ошибка при валидации регистрационных данных", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
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

	tokenString, err := auth.BuildJWTToken(&user, h.JWTSecretKey)
	if err != nil {
		logger.Log.Debug("Ошибка при создании JWT-токена", logger.String("jwt-token", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при создании JWT-токена")
		return
	}

	auth.CreateCookie(w, tokenString)

	logger.Log.Debug("Успешная регистрация пользователя", logger.String("login", user.Login))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response.AuthResponse{
		Message: "Пользователь зарегистрирован",
		Login:   user.Login,
		Token:   tokenString,
	})
	if err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
