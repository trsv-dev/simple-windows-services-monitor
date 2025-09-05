package models

import "time"

// Server Модель сервера.
type Server struct {
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"created_at"`
	Services  []Service `json:"services"`
}

// Service Модель службы.
type Service struct {
	DisplayedName string `json:"displayedName"`
	ServiceName   string `json:"serviceName"`
	Status        string `json:"status"`
}

// User Модель пользователя.
type User struct {
	Login    string   `json:"login"`
	Password string   `json:"password"`
	Servers  []Server `json:"servers"`
}
