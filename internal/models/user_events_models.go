package models

// KeycloakUserEvent Модель для хранения данных UserEvent ("REGISTER", "LOGIN", "LOGOUT", "CODE_TO_TOKEN" и т.д.).
type KeycloakUserEvent struct {
	Type      string            `json:"type"` // "REGISTER", "LOGIN", "LOGOUT", "CODE_TO_TOKEN", etc.
	RealmID   string            `json:"realmId"`
	ClientID  string            `json:"clientId"`
	UserID    string            `json:"userId"`
	SessionID string            `json:"sessionId"`
	IPAddress string            `json:"ipAddress"`
	Time      int64             `json:"time"`
	Details   map[string]string `json:"details"`
	Error     string            `json:"error,omitempty"`
}

// UserRepresentation Модель для хранения распарсенных данных пользователя.
type UserRepresentation struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
