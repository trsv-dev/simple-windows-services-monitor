package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServerStorage Интерфейс для серверов.
type ServerStorage interface {
	AddServer(ctx context.Context, server models.Server, userID int64) (*models.Server, error)
	EditServer(ctx context.Context, input *models.Server, serverID int64, userID int64) (*models.Server, error)
	DelServer(ctx context.Context, serverID int64, userID int64) error
	GetServer(ctx context.Context, serverID int64, userID int64) (*models.Server, error)
	GetServerWithPassword(ctx context.Context, serverID int64, userID int64) (*models.Server, error)
	ListServers(ctx context.Context, userID int64) ([]*models.Server, error)
}
