package storage

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// ServiceStorage Интерфейс для служб.
type ServiceStorage interface {
	AddService(ctx context.Context, serverID int64, userID int64, service models.Service) (*models.Service, error)
	DelService(ctx context.Context, serverID int64, serviceID int64, userID int64) error
	ChangeServiceStatus(ctx context.Context, serverID int64, serviceName string, status string) error
	BatchChangeServiceStatus(ctx context.Context, serverID int64, servicesBatch []*models.Service) error
	GetService(ctx context.Context, serverID int64, serviceID int64, userID int64) (*models.Service, error)
	ListServices(ctx context.Context, serverID int64, userID int64) ([]*models.Service, error)
}
