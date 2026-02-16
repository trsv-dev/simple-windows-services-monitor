package models

import (
	"errors"
	"time"
)

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

// ServiceName Модель с названием и отображаемым именем службы для отдачи клиенту.
type ServiceName struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}
