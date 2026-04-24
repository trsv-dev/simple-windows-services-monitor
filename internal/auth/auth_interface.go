package auth

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth/keycloak/models"
)

//go:generate mockgen -destination=mocks/mock_auth_provider.go -package=mocks . AuthProvider

// AuthProvider Интерфейс авторизации.
type AuthProvider interface {
	ValidateToken(ctx context.Context, token string) (*models.UserClaims, error)
}
