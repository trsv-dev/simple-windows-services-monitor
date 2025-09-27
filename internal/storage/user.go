package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// UserStorage Интерфейс для пользователей.
type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUserIDByLogin(ctx context.Context, login string) (int, error)
	ListUsers(ctx context.Context) ([]*models.User, error)
	GetUserServiceStatuses(ctx context.Context, userID int64) ([]*models.ServiceStatus, error)
}
