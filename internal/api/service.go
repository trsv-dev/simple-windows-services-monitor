package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/utils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/worker"
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

	rawServiceName := service.ServiceName
	service.ServiceName = strings.ToLower(strings.TrimSpace(strings.Trim(service.ServiceName, "\"'`«»“”‘’")))

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

	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !netutils.IsHostReachable(server.Address, 5985, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен. Невозможно добавить службу", server.Address, server.ID))

		w.Header().Set("Content-Type", "application/json")
		response.ErrorJSON(w, http.StatusBadGateway, "Сервер недоступен")
		return
	}

	// создаём WinRM клиент
	client, err := service_control.NewWinRMClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
		return
	}

	statusCmd := fmt.Sprintf("sc query \"%s\"", service.ServiceName)

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

	// проверяем, существует ли вообще такая служба на сервере
	if !isServiceExists(result) {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s` на сервере `%s`, address=%s, id=%d",
			service.DisplayedName, server.Name, server.Address, creds.ServerID))

		response.ErrorJSON(w, http.StatusNotFound, fmt.Sprintf("Служба `%s` не найдена на сервере", rawServiceName))
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
	// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
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

	if len(services) == 0 {
		services = []*models.Service{}
	}

	// если запрос пришел без параметра ?actual=true - просто временем каждую службу в списке
	// если служб у сервера нет - возвращаем пустой срез служб
	// если запрос пришел с параметром ?actual=true, но служб нет - вернем пустой массив
	if r.URL.Query().Get("actual") != "true" || len(services) == 0 {
		w.Header().Set("Content-Type", "application/json")
		// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
		if err = json.NewEncoder(w).Encode(services); err != nil {
			logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			return
		}
		return
	}

	// если запрос пришел с параметром ?actual=true - сначала получаем актуальные статусы служб с сервера
	// и обновляем новым статусом и временем каждую службу в списке

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

	// проверяем доступность сервера, если недоступен - возвращаем службы и заголовок "X-Is-Updated" = false
	if !netutils.IsHostReachable(server.Address, 5985, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен. Невозможно обновить статус служб с сервера", server.Address, server.ID))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Is-Updated", "false")
		// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
		if err = json.NewEncoder(w).Encode(services); err != nil {
			logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			return
		}
		return
	}

	// опрашиваем службы через воркер, получаем слайс служб с обновленными данными и булево значение об успехе
	updates, success := worker.CheckServicesStatuses(ctx, server, services)

	if !success {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Is-Updated", "false")
		// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
		if err = json.NewEncoder(w).Encode(services); err != nil {
			logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			return
		}
		return
	}

	err = h.storage.BatchChangeServiceStatus(ctx, creds.ServerID, updates)
	if err != nil {
		logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))

		w.Header().Set("X-Is-Updated", "false")
		// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
		if err = json.NewEncoder(w).Encode(services); err != nil {
			logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
			return
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Is-Updated", "true")
	// не выставляем w.WriteHeader(http.StatusOK), т.к. NewEncoder(w).Encode() сам вернет http.StatusOK
	if err = json.NewEncoder(w).Encode(services); err != nil {
		logger.Log.Error("Ошибка кодирования JSON", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Внутренняя ошибка сервера")
		return
	}
}

// Вспомогательная функция проверки результата запроса на получение статуса службы
func isServiceExists(result string) bool {
	// если явно присутствует код 1060 - служба отсутствует
	if strings.Contains(result, "1060") {
		return false
		// искомые маркеры наличия службы
	} else if strings.Contains(result, "STATE") || strings.Contains(result, "SERVICE_NAME:") {
		return true
	}

	// если ничего не понятно - считаем что такой службы нет на сервере
	return false
}
