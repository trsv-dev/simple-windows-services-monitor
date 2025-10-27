package auth

import (
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

//go:generate mockgen -destination=mocks/mock_token_builder.go -package=mocks . TokenBuilder

// TokenBuilder Интерфейс для создания и парсинга JWT-токенов.
type TokenBuilder interface {
	BuildJWTToken(user *models.User, JWTSecretKey string) (string, error)
	GetClaims(tokenString, JWTSecretKey string) (*Claims, error)
}
