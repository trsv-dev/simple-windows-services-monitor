package models

import "strings"

// KeycloakAdminEvent Модель для хранения данных AdminEvent ("CREATE", "UPDATE", "DELETE")
type KeycloakAdminEvent struct {
	OperationType  string `json:"operationType"` // "CREATE", "UPDATE", "DELETE"
	ResourceType   string `json:"resourceType"`  // "USER", "CLIENT", etc.
	ResourcePath   string `json:"resourcePath"`  // "users/xxx-xxx-xxx"
	RealmID        string `json:"realmId"`
	Time           int64  `json:"time"`
	Representation string `json:"representation"`
}

// ExtractUserID Вспомогательная функция. Извлекает UUID пользователя из resourcePath.
func (ae *KeycloakAdminEvent) ExtractUserID() string {
	parts := strings.Split(ae.ResourcePath, "/")

	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}
