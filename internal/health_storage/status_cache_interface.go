package health_storage

import "github.com/trsv-dev/simple-windows-services-monitor/internal/models"

//go:generate mockgen -destination=mocks/status_cache_storage_mock.go -package=mocks . StatusCacheStorage

type StatusCacheStorage interface {
	Set(s models.ServerStatus)
	Get(id int64) (models.ServerStatus, bool)
	Delete(id int64)
}
