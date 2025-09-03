package storage

import "github.com/trsv-dev/simple-windows-services-monitor/internal/models"

// ServerStorage Интерфейс для серверов.
type ServerStorage interface {
	AddServer(server models.Server) error
	DelServer(srvAddr string) error
	GetServer(srvAddr string) (models.Server, error)
	ListServers() ([]models.Server, error)
}
