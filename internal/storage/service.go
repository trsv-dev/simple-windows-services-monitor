package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServiceStorage Интерфейс для служб.
type ServiceStorage interface {
	AddService(ctx context.Context, srvAddr string, service models.Service) error
	DelService(ctx context.Context, srvAddr string, service models.Service) error
	GetService(ctx context.Context, srvAddr string) (models.Service, error)
	ListServices(ctx context.Context, srvAddr string) ([]models.Service, error)
}
