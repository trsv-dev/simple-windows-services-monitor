package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServerStorage Интерфейс для серверов.
type ServerStorage interface {
	AddServer(ctx context.Context, server models.Server, userID int) error
	EditServer(ctx context.Context, id int, login string, input models.Server) (*models.Server, error)
	DelServer(ctx context.Context, id int, login string) error
	GetServer(ctx context.Context, id int, login string) (*models.Server, error)
	ListServers(ctx context.Context, login string) ([]models.Server, error)
}
