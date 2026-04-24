package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// UserStorage Интерфейс для пользователей.
type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, userID string) error
	UserExists(ctx context.Context, userID string) (bool, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	GetUserServiceStatuses(ctx context.Context, userID string) ([]*models.ServiceStatus, error)
}
