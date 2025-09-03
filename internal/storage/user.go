package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// UserStorage Интерфейс для служб.
type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUser(ctx context.Context, user *models.User) (*models.User, error)
}
