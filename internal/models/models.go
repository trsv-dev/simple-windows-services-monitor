package models

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/utils"
)

const (
	loginLen      = 4
	passwordLen   = 5
	serverNameLen = 3
)

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

// Service Модель службы.
type Service struct {
	ID            int64     `json:"id,omitempty"`
	DisplayedName string    `json:"displayed_name"`
	ServiceName   string    `json:"service_name"`
	Status        string    `json:"status,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// Validate Базовая валидация данных.
func (s Service) Validate() error {
	if len(s.DisplayedName) == 0 {
		return errors.New("необходимо указать отображаемое имя")
	}

	if len(s.ServiceName) == 0 {
		return errors.New("необходимо указать имя службы в Windows")
	}

	return nil
}

// ServiceStatus Модель статуса службы.
type ServiceStatus struct {
	ID        int64     `json:"id"`
	ServerID  int64     `json:"server_id"`
	Status    string    `json:"status,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ServerStatus Модель статуса сервера.
type ServerStatus struct {
	ServerID int64  `json:"server_id"`
	UserID   int64  `json:"user_id"`
	Address  string `json:"address"`
	Status   string `json:"status,omitempty"`
}

// ValidateStatus Валидация статусов сервера.
func (s *ServerStatus) ValidateStatus(serverStatus string) error {
	switch serverStatus {
	case "OK", "Degraded", "Unreachable":
		return nil
	default:
		return errors.New("неожиданный статус сервера")
	}
}

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
