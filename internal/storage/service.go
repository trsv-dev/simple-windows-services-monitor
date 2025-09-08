package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServiceStorage Интерфейс для служб.
type ServiceStorage interface {
	AddService(ctx context.Context, serverID int, login string, service models.Service) error
	DelService(ctx context.Context, serverID int, serviceID int, login string) error
	GetService(ctx context.Context, serverID int, serviceID int, login string) (*models.Service, error)
	ListServices(ctx context.Context, serverID int, login string) ([]models.Service, error)
}
