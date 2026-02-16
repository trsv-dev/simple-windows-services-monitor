package models

import (
	"errors"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/utils"
)

const (
	loginLen    = 4
	passwordLen = 5
)

// RegisterRequest Модель для тела запроса регистрации пользователя.
type RegisterRequest struct {
	ID              int64  `json:"id,omitempty"`
	Login           string `json:"login"`
	Password        string `json:"password"`
	RegistrationKey string `json:"registration_key,omitempty"`
}

// User Модель пользователя.
type User struct {
	ID       int64  `json:"id,omitempty"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Validate Базовая валидация данных.
func (u User) Validate() error {
	if len(u.Login) < loginLen {
		return errors.New("передан слишком короткий логин (менее 4 символов)")
	}

	if len(u.Password) < passwordLen {
		return errors.New("передан слишком короткий пароль (менее 5 символов)")
	}

	if !utils.IsAlphaNumericOrSpecial(u.Login) {
		return errors.New("недопустимые символы в логине")
	}

	if !utils.IsAlphaNumericOrSpecial(u.Password) {
		return errors.New("недопустимые символы в пароле")
	}

	return nil
}
