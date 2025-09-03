package storage

import "github.com/trsv-dev/simple-windows-services-monitor/internal/models"

// ServiceStorage Интерфейс для служб.
type ServiceStorage interface {
	AddService(srvAddr string, service models.Service) error
	DelService(srvAddr string, service models.Service) error
	GetService(srvAddr string) (models.Service, error)
	ListServices(srvAddr string) ([]models.Service, error)
}
