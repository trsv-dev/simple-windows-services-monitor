package auth

import "context"

//go:generate mockgen -destination=mocks/mock_auth_provider.go -package=mocks . AuthProvider

// UserClaims Структура интерфейса авторизации.
type UserClaims struct {
	ID    string
	Login string
}

// AuthProvider Интерфейс авторизации.
type AuthProvider interface {
	ValidateToken(ctx context.Context, token string) (*UserClaims, error)
}
