package models

// Claims - минимальный набор OIDC/JWT клеймов,
// необходимых приложению для идентификации пользователя.
// Используется как промежуточная структура при разборе токена.
type Claims struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
}

// UserClaims - доменная модель аутентифицированного пользователя,
// используемая внутри приложения (не зависит от конкретного провайдера).
type UserClaims struct {
	ID    string
	Login string
}
