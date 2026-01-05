package worker

import (
	"context"
	"fmt"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
)

func ServerStatusWorker(ctx context.Context, checker netutils.Checker, server *models.Server, winrmPort string) chan models.ServerStatus {
	statusCh := make(chan models.ServerStatus, 1)
	defer close(statusCh)

	for {
		select {
		case <-ctx.Done():
			logger.Log.Error("Канал получения статуса сервера закрыт по контексту")
			status := models.ServerStatus{ServerID: server.ID, Address: server.Address, Status: "Unreachable"}
			statusCh <- status
			//close(statusCh)
		default:
			// проверяем доступность сервера, если недоступен - возвращаем ошибку
			if !checker.IsHostReachable(ctx, server.Address, winrmPort, 0) {
				logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен", server.Address, server.ID))
				status := models.ServerStatus{ServerID: server.ID, Address: server.Address, Status: "Unreachable"}
				statusCh <- status

				return statusCh
			}

			status := models.ServerStatus{ServerID: server.ID, Address: server.Address, Status: "OK"}
			statusCh <- status

			return statusCh
		}
	}
}
