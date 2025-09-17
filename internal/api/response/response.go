package response

import (
	"encoding/json"
	"net/http"
)

// AuthResponse Успешный ответ при регистрации или авторизации пользователя.
type AuthResponse struct {
	Message string `json:"message"`
	Login   string `json:"login"`
	Token   string `json:"token"`
}

// APIError Модель возвращаемых ответов при ошибках.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APISuccess Модель успешного ответа API.
type APISuccess struct {
	Message string `json:"message"`
}

// JSON Пишет в ответ хендлера произвольные данные.
func JSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// SuccessJSON Шаблон для успешного ответа в хендлерах.
func SuccessJSON(w http.ResponseWriter, status int, message string) {
	JSON(w, status, APISuccess{Message: message})
}

// ErrorJSON Шаблон для ответа с ошибкой в хендлерах.
func ErrorJSON(w http.ResponseWriter, status int, message string) {
	JSON(w, status, APIError{Code: status, Message: message})
}
