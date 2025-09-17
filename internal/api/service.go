package api

import (
	"encoding/json"
	"errors"
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
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)

	var service models.Service

	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err := service.Validate(); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	createdService, err := h.storage.AddService(ctx, serverID, login, service)
	var ErrDuplicatedService *errs.ErrDuplicatedService
	var ErrServerNotFound *errs.ErrServerNotFound
	if err != nil {
		switch {
		case errors.As(err, &ErrDuplicatedService):
			logger.Log.Error("Дубликат службы", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusConflict, "Служба уже была добавлена")
			return
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Error("Сервер не был найден", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не был найден")
			return
		default:
			logger.Log.Error("Ошибка добавления службы в БД", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка добавления службы")
			return
		}
	}

	logger.Log.Debug("Служба успешно добавлена на сервер", logger.String("serviceName", service.ServiceName), logger.Int("serverID", serverID))
	//response.SuccessJSON(w, http.StatusOK, "Служба успешно добавлена")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err = json.NewEncoder(w).Encode(createdService); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// DelService Удаление службы.
func (h *AppHandler) DelService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)
	serviceID := ctx.Value(contextkeys.ServiceID).(int)

	err := h.storage.DelService(ctx, serverID, serviceID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound
	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Error("Служба не найдена", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка удаления службы", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка удаления службы")
			return
		}
	}

	logger.Log.Debug("Служба успешно удалена", logger.Int("serverID", serverID), logger.Int("serviceID", serviceID))
	response.SuccessJSON(w, http.StatusOK, "Служба успешно удалена")
}

// GetService Получение информации о службе.
func (h *AppHandler) GetService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)
	serviceID := ctx.Value(contextkeys.ServiceID).(int)

	service, err := h.storage.GetService(ctx, serverID, serviceID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(service); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// GetServicesList Получение списка служб сервера, принадлежащего пользователю.
func (h *AppHandler) GetServicesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)

	services, err := h.storage.ListServices(ctx, serverID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Error("Сервер не был найден", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не был найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении списка служб сервера", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении списка служб")
			return
		}
	}

	// если служб у сервера нет - возвращаем пустой срез служб
	if len(services) == 0 {
		services = []*models.Service{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(services); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
