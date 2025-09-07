package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// AddService Добавление службы.
func (h *AppHandler) AddService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	//login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.IDKey).(int)

	var service models.Service

	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		fmt.Println(err)
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	err := h.storage.AddService(ctx, serverID, service)
	var ErrDuplicatedService *errs.ErrDuplicatedService
	if err != nil {
		switch {
		case errors.As(err, &ErrDuplicatedService):
			logger.Log.Error("Дубликат службы", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Служба уже была добавлена")
			return
		default:
			logger.Log.Error("Ошибка добавления службы в БД", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка добавления службы")
			return
		}
	}

	logger.Log.Debug("Служба успешно добавлена на сервер", logger.String("serviceName", service.ServiceName), logger.Int("serverID", serverID))
	response.SuccessJSON(w, http.StatusOK, "Служба успешно добавлена")
}
