package models

// User Модель пользователя.
type User struct {
	ID    string `json:"id,omitempty"`
	Login string `json:"login"`
}
