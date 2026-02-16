package models

type Status string

const (
	StatusOK          Status = "OK"
	StatusDegraded    Status = "Degraded"
	StatusUnreachable Status = "Unreachable"
	StatusUnknown     Status = "Unknown"
)

// IsValid Валидация статуса.
func (s Status) IsValid() bool {
	switch s {
	case StatusOK, StatusDegraded, StatusUnreachable, StatusUnknown:
		return true
	default:
		return false
	}
}

// String Стрингер для Status.
func (s Status) String() string {
	return string(s)
}

// ServerStatus Модель статуса сервера.
type ServerStatus struct {
	ServerID int64  `json:"server_id"`
	UserID   int64  `json:"user_id"`
	Address  string `json:"address"`
	Status   Status `json:"status"`
}
