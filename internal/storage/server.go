package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServerStorage Интерфейс для серверов.
type ServerStorage interface {
	AddServer(ctx context.Context, server models.Server) error
	DelServer(ctx context.Context, srvAddr string) error
	GetServer(ctx context.Context, srvAddr string) (models.Server, error)
	ListServers(ctx context.Context) ([]models.Server, error)
}
