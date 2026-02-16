package models

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/google/uuid"
)

const serverNameLen = 3

// Server Модель сервера.
type Server struct {
	ID          int64     `json:"id,omitempty"`
	Name        string    `json:"name"`
	Address     string    `json:"address"`
	Username    string    `json:"username"`
	Password    string    `json:"password,omitempty"`
	Fingerprint uuid.UUID `json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateValidation Базовая валидация данных при создании сервера.
func (s Server) CreateValidation() error {
	if len(s.Name) == 0 {
		return errors.New("необходимо указать имя сервера (минимум 3 символа)")
	}

	if len(s.Address) == 0 {
		return errors.New("необходимо указать адрес сервера")
	}

	if len(s.Username) == 0 {
		return errors.New("необходимо указать логин")
	}

	if len(s.Password) == 0 {
		return errors.New("необходимо указать пароль")
	}

	addr := s.Address

	// проверка на IP
	isDigitsAndDots := regexp.MustCompile(`^[0-9.]+$`).MatchString(addr)
	if isDigitsAndDots {
		if net.ParseIP(addr) == nil {
			return fmt.Errorf("невалидный IP адрес: %s", addr)
		}
		return nil
	}

	// проверка hostname (короткие имена и домены)
	if len(addr) > 253 { // RFC максимум для полного домена
		return fmt.Errorf("hostname слишком длинный: %s", addr)
	}

	hostnameRegex := regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*$`)
	if !hostnameRegex.MatchString(addr) {
		return fmt.Errorf("невалидный hostname: %s", addr)
	}

	return nil
}

// UpdateValidation Базовая валидация данных при редактировании сервера.
func (s Server) UpdateValidation() error {
	if s.Name != "" {
		if len(s.Name) < serverNameLen {
			return errors.New("необходимо указать имя сервера (минимум 3 символа)")
		}
	}

	if s.Address != "" {
		addr := s.Address
		isDigitsAndDots := regexp.MustCompile(`^[0-9.]+$`).MatchString(addr)
		if isDigitsAndDots {
			if net.ParseIP(addr) == nil {
				return fmt.Errorf("невалидный IP адрес: %s", addr)
			}
		} else {
			if len(addr) > 253 {
				return fmt.Errorf("hostname слишком длинный: %s", addr)
			}
			hostnameRegex := regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(?:\.(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?))*$`)
			if !hostnameRegex.MatchString(addr) {
				return fmt.Errorf("невалидный hostname: %s", addr)
			}
		}
	}

	if s.Username != "" && len(s.Username) == 0 {
		return errors.New("необходимо указать логин")
	}

	if s.Password != "" && len(s.Password) == 0 {
		return errors.New("необходимо указать пароль")
	}

	return nil
}
