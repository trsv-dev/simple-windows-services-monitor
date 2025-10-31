package registration_handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// RegistrationHandler Обработчик регистрации.
type RegistrationHandler struct {
	storage          storage.Storage
	tokenBuilder     auth.TokenBuilder
	JWTSecretKey     string
	registrationKey  string
	openRegistration bool
}

// NewRegistrationHandler Конструктор RegistrationHandler.
func NewRegistrationHandler(storage storage.Storage, tokenBuilder auth.TokenBuilder, JWTSecretKey string, registrationKey string, openRegistration bool) *RegistrationHandler {
	return &RegistrationHandler{
		storage:          storage,
		tokenBuilder:     tokenBuilder,
		JWTSecretKey:     JWTSecretKey,
		registrationKey:  registrationKey,
		openRegistration: openRegistration,
	}
}

// UserRegistration Регистрация пользователей.
func (h *RegistrationHandler) UserRegistration(w http.ResponseWriter, r *http.Request) {
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

	// registerRequest включает в себя поля модели models.User и поле RegistrationKey, необходимое для
	// успешной регистрации если включена ограниченная регистрация пользователей
	var registerRequest models.RegisterRequest

	err = json.Unmarshal(data, &registerRequest)
	if err != nil {
		logger.Log.Error("Ошибка декодирования тела запроса при регистрации", logger.String("error", err.Error()))
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	// если открытая регистрация выключена в .env - для регистрации потребуется регистрационный ключ,
	// проверяем поступивший в запросе ключ на соответствие ключу из .env
	if !h.openRegistration {
		if registerRequest.RegistrationKey != h.registrationKey {
			logger.Log.Error("Попытка регистрации с невалидным ключом")
			response.ErrorJSON(w, http.StatusBadRequest, fmt.Errorf("невалидный ключ регистрации").Error())
			return
		}
	}

	// если открытая регистрация включена в .env - ключ не требуется, просто продолжаем
	user := models.User{
		Login:    registerRequest.Login,
		Password: registerRequest.Password,
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
			logger.Log.Info("Такой пользователь уже существует",
				logger.String("login", ErrLoginIsTaken.Login),
				logger.String("err", ErrLoginIsTaken.Err.Error()))
			response.ErrorJSON(w, http.StatusConflict, "Пользователь уже существует")
			return
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	tokenString, err := h.tokenBuilder.BuildJWTToken(&user, h.JWTSecretKey)
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
