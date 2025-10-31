package worker

import (
	"context"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

//go:generate mockgen -destination=mocks/mock_statuses_checker.go -package=mocks . StatusesChecker

// StatusesChecker Интерфейс для проверки статусов служб на сервере.
type StatusesChecker interface {
	CheckServicesStatuses(ctx context.Context, server *models.Server, services []*models.Service) ([]*models.Service, bool)
}
