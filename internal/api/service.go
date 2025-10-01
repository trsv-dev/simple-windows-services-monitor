package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/utils"
)

// AddService Добавление службы.
func (h *AppHandler) AddService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	var service models.Service

	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, "Неверный формат запроса")
		return
	}

	if err := service.Validate(); err != nil {
		response.ErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	server, err := h.storage.GetServerWithPassword(ctx, creds.ServerID, creds.UserID)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServerNotFound.UserID),
				logger.Int64("serverID", ErrServerNotFound.ServerID),
				logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	// создаём WinRM клиент
	client, err := service_control.NewWinRMClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
		return
	}

	statusCmd := fmt.Sprintf("sc query %s", service.ServiceName)

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s` на сервере `%s`, id=%d",
			service.DisplayedName, server.Name, creds.ServerID), logger.String("err", err.Error()))

		response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Не удалось получить статус службы `%s`", service.DisplayedName))
		return
	}

	statusFromServer := utils.GetStatusByINT(utils.GetStatus(result))
	service.Status = statusFromServer
	service.UpdatedAt = time.Now()

	createdService, err := h.storage.AddService(ctx, creds.ServerID, creds.UserID, service)
	var ErrDuplicatedService *errs.ErrDuplicatedService
	var ErrServerNotFound2 *errs.ErrServerNotFound
	if err != nil {
		switch {
		case errors.As(err, &ErrDuplicatedService):
			logger.Log.Error("Дубликат службы",
				logger.Int64("serverID", creds.ServerID),
				logger.String("serviceName", service.ServiceName),
				logger.String("err", ErrDuplicatedService.Err.Error()))
			response.ErrorJSON(w, http.StatusConflict, "Служба уже была добавлена")
			return
		case errors.As(err, &ErrServerNotFound2):
			logger.Log.Error("Сервер не был найден", logger.String("err", ErrServerNotFound2.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не был найден")
			return
		default:
			logger.Log.Error("Ошибка добавления службы в БД", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка добавления службы")
			return
		}
	}

	logger.Log.Debug("Служба успешно добавлена на сервер", logger.String("serviceName", service.ServiceName), logger.Int64("serverID", creds.ServerID))
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
	creds := models.GetContextCreds(ctx)

	err := h.storage.DelService(ctx, creds.ServerID, creds.ServiceID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound
	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Error("Служба не найдена",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServiceNotFound.UserID),
				logger.Int64("serverID", ErrServiceNotFound.ServerID),
				logger.String("err", ErrServiceNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка удаления службы", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка удаления службы")
			return
		}
	}

	logger.Log.Debug("Служба успешно удалена", logger.Int64("serverID", creds.ServerID), logger.Int64("serviceID", creds.ServiceID))
	response.SuccessJSON(w, http.StatusOK, "Служба успешно удалена")
}

// GetService Получение информации о службе.
func (h *AppHandler) GetService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	service, err := h.storage.GetService(ctx, creds.ServerID, creds.ServiceID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена",
				logger.String("login", creds.Login),
				logger.Int64("userID", ErrServiceNotFound.UserID),
				logger.Int64("serverID", ErrServiceNotFound.ServerID),
				logger.Int64("serviceID", ErrServiceNotFound.ServiceID),
				logger.String("err", ErrServiceNotFound.Err.Error()))
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
	creds := models.GetContextCreds(ctx)

	services, err := h.storage.ListServices(ctx, creds.ServerID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Error("Сервер не был найден",
				logger.String("login", creds.Login),
				logger.String("err", ErrServiceNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не был найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении списка служб сервера",
				logger.String("login", creds.Login),
				logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении списка служб")
			return
		}
	}

	// если служб у сервера нет - возвращаем пустой срез служб
	if len(services) == 0 {
		services = []*models.Service{}
	}

	// если запрос пришел с параметром ?actual=true - сначала получаем актуальные статусы служб с сервера
	// и обновляем новым статусом и временем каждую службу в списке
	if r.URL.Query().Get("actual") == "true" && len(services) != 0 {
		server, err := h.storage.GetServerWithPassword(ctx, creds.ServerID, creds.UserID)

		var ErrServerNotFound *errs.ErrServerNotFound

		if err != nil {
			switch {
			case errors.As(err, &ErrServerNotFound):
				logger.Log.Warn("Сервер не найден",
					logger.String("login", creds.Login),
					logger.Int64("userID", ErrServerNotFound.UserID),
					logger.Int64("serverID", ErrServerNotFound.ServerID),
					logger.String("err", ErrServerNotFound.Err.Error()))
				response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
				return
			default:
				logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
				response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
				return
			}
		}

		// создаём WinRM клиент
		client, err := service_control.NewWinRMClient(server.Address, server.Username, server.Password)

		if err != nil {
			logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
			return
		}

		// получаем актуальный статус службы с сервера посредством WinRM
		for _, service := range services {
			statusCmd := fmt.Sprintf("sc query %s", service.ServiceName)

			// контекст для получения статуса
			statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

			result, err := client.RunCommand(statusCtx, statusCmd)
			cancel()

			if err != nil {
				logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s` на сервере `%s`, id=%d",
					service.DisplayedName, server.Name, creds.ServerID), logger.String("err", err.Error()))

				//response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Не удалось получить статус службы `%s`", service.DisplayedName))
				continue
			}

			statusFromServer := utils.GetStatusByINT(utils.GetStatus(result))
			service.Status = statusFromServer
			service.UpdatedAt = time.Now()

			err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, statusFromServer)
			if err != nil {
				logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
				continue
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err = json.NewEncoder(w).Encode(services); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}
