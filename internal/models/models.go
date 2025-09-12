package models

import "time"

// Server Модель сервера.
type Server struct {
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Service Модель службы.
type Service struct {
	DisplayedName string    `json:"displayed_name"`
	ServiceName   string    `json:"service_name"`
	Status        string    `json:"status"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// User Модель пользователя.
type User struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
