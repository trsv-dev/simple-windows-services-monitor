package control_handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/netutils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/utils"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage"
)

// ControlHandler Обрабатывает запросы управления службами (start, stop, restart, status).
type ControlHandler struct {
	storage       storage.Storage
	clientFactory service_control.ClientFactory // фабрика для создания WinRM клиентов
	checker       netutils.Checker
	winrmPort     string
}

// NewControlHandler Конструктор ControlHandler.
func NewControlHandler(storage storage.Storage, clientFactory service_control.ClientFactory, checker netutils.Checker, winrmPort string) *ControlHandler {
	return &ControlHandler{
		storage:       storage,
		clientFactory: clientFactory,
		checker:       checker,
		winrmPort:     winrmPort,
	}
}

// ServiceStop Остановка службы.
func (h *ControlHandler) ServiceStop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	// получаем сервер с паролем
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

	// получаем службу
	service, err := h.storage.GetService(ctx, creds.ServerID, creds.ServiceID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", ErrServiceNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
			return
		}
	}

	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !h.checker.CheckWinRM(ctx, server.Address, h.winrmPort, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен. Невозможно остановить службу", server.Address, server.ID))
		response.ErrorJSON(w, http.StatusBadGateway, fmt.Sprintf("Сервер недоступен"))
		return
	}

	// создаём WinRM клиент
	client, err := h.clientFactory.CreateClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
		return
	}

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	statusCmd := fmt.Sprintf("sc query \"%s\"", service.ServiceName)
	stopCmd := fmt.Sprintf("sc stop \"%s\"", service.ServiceName)

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))

		response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Не удалось получить статус службы `%s`", service.DisplayedName))
		return
	}

	status := utils.GetStatus(result)

	switch status {
	case utils.ServiceRunning, utils.ServiceStartPending:
		// пробуем остановить

		// контекст для остановки
		stopCtx, cancelStop := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStop()

		var stdout string

		// получаем вывод после выполнения команды остановки
		if stdout, err = client.RunCommand(stopCtx, stopCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Не удалось остановить службу")
			return
		}

		// проверяем вывод на FAILED, .т.е на ошибку остановки службы
		// (если exit code != 0, то в выводе точно будет FAILED)
		if serviceErr := errs.ParseServiceError(stdout); serviceErr != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", serviceErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, serviceErr.Error())
			return
		}

		// контекст для ожидания остановки
		waitStopCtx, cancelWaitStop := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitStop()

		// ждём остановки с контекстом и экспоненциальной задержкой
		if waitErr := h.waitForServiceStatus(waitStopCtx, client, service.ServiceName, utils.ServiceStopped); waitErr != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не остановилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешной остановки
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
			// не возвращаем ошибку пользователю, т.к. служба реально остановлена
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` остановлена", service.DisplayedName))

	case utils.ServiceStopped:
		// уже остановлена

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d уже остановлена",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` уже остановлена", service.DisplayedName))

	case utils.ServiceStopPending, utils.ServicePausePending:
		// уже выполняется остановка/пауза
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже останавливается на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже останавливается", service.DisplayedName))

	default:
		// неожиданный статус
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем остановку",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем остановку", service.DisplayedName))

	}
}

// ServiceStart Запуск службы.
func (h *ControlHandler) ServiceStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	// получаем сервер с паролем
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

	// получаем службу
	service, err := h.storage.GetService(ctx, creds.ServerID, creds.ServiceID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", ErrServiceNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
			return
		}
	}

	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !h.checker.CheckWinRM(ctx, server.Address, h.winrmPort, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен. Невозможно запустить службу", server.Address, server.ID))
		response.ErrorJSON(w, http.StatusBadGateway, fmt.Sprintf("Сервер недоступен"))
		return
	}

	// создаём WinRM клиент
	client, err := h.clientFactory.CreateClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
		return
	}

	statusCmd := fmt.Sprintf("sc query \"%s\"", service.ServiceName)
	startCmd := fmt.Sprintf("sc start \"%s\"", service.ServiceName)

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))

		response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Не удалось получить статус службы `%s`", service.DisplayedName))
		return
	}

	status := utils.GetStatus(result)

	switch status {
	case utils.ServiceStopped, utils.ServiceStopPending:
		// пробуем запустить

		// контекст для запуска
		startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStart()

		var stdout string

		// получаем вывод после выполнения команды запуска
		if stdout, err = client.RunCommand(startCtx, startCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Не удалось запустить службу")
			return
		}

		// проверяем вывод на FAILED, .т.е на ошибку запуска службы
		// (если exit code != 0, то в выводе точно будет FAILED)
		if serviceErr := errs.ParseServiceError(stdout); serviceErr != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", serviceErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, serviceErr.Error())
			return
		}

		// контекст для ожидания запуска
		waitStartCtx, cancelWaitStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitStart()

		// ждём запуска с контекстом и экспоненциальной задержкой
		if waitErr := h.waitForServiceStatus(waitStartCtx, client, service.ServiceName, utils.ServiceRunning); waitErr != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не запустилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешного запуска
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` запущена", service.DisplayedName))

	case utils.ServiceRunning:
		// уже запущена

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d уже запущена",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` уже запущена", service.DisplayedName))

	case utils.ServiceStartPending, utils.ServicePausePending:
		// уже выполняется запуск/пауза
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже запускается на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже запускается", service.DisplayedName))

	default:
		// неожиданный статус
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем запуск",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем запуск", service.DisplayedName))
	}
}

// ServiceRestart Перезапуск службы.
func (h *ControlHandler) ServiceRestart(w http.ResponseWriter, r *http.Request) {
	var stdout string

	ctx := r.Context()
	creds := models.GetContextCreds(ctx)

	// получаем сервер с паролем
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

	// получаем службу
	service, err := h.storage.GetService(ctx, creds.ServerID, creds.ServiceID, creds.UserID)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", ErrServiceNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
			return
		}
	}

	// проверяем доступность сервера, если недоступен - возвращаем ошибку
	if !h.checker.CheckWinRM(ctx, server.Address, h.winrmPort, 0) {
		logger.Log.Warn(fmt.Sprintf("Сервер %s, id=%d недоступен. Невозможно перезапустить службу", server.Address, server.ID))
		response.ErrorJSON(w, http.StatusBadGateway, fmt.Sprintf("Сервер недоступен"))
		return
	}

	// создаём WinRM клиент
	client, err := h.clientFactory.CreateClient(server.Address, server.Username, server.Password)

	if err != nil {
		logger.Log.Error("Ошибка создания WinRM клиента", logger.String("err", err.Error()))
		response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка подключения к серверу")
		return
	}

	statusCmd := fmt.Sprintf("sc query \"%s\"", service.ServiceName)
	stopCmd := fmt.Sprintf("sc stop \"%s\"", service.ServiceName)
	startCmd := fmt.Sprintf("sc start \"%s\"", service.ServiceName)

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))

		response.ErrorJSON(w, http.StatusInternalServerError, fmt.Sprintf("Не удалось получить статус службы `%s`", service.DisplayedName))
		return
	}

	status := utils.GetStatus(result)

	switch status {
	case utils.ServiceRunning:
		// сначала пробуем остановить

		// контекст для остановки
		stopCtx, cancelStop := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStop()

		if stdout, err = client.RunCommand(stopCtx, stopCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось остановить службу `%s`", service.DisplayedName))
			return
		}

		// проверяем вывод на FAILED, .т.е на ошибку остановки службы
		// (если exit code != 0, то в выводе точно будет FAILED)
		if serviceErr := errs.ParseServiceError(stdout); serviceErr != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", serviceErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, serviceErr.Error())
			return
		}

		// контекст для ожидания остановки
		waitStopCtx, cancelWaitStop := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitStop()

		// ждём остановки с контекстом и экспоненциальной задержкой
		if waitErr := h.waitForServiceStatus(waitStopCtx, client, service.ServiceName, utils.ServiceStopped); waitErr != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не остановилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешной остановки
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		// теперь запускаем

		// контекст для запуска
		startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStart()

		if stdout, err = client.RunCommand(startCtx, startCmd); err != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось запустить службу `%s`", service.DisplayedName))
			return
		}

		// проверяем вывод на FAILED, .т.е на ошибку запуска службы
		// (если exit code != 0, то в выводе точно будет FAILED)
		if serviceErr := errs.ParseServiceError(stdout); serviceErr != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", serviceErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, serviceErr.Error())
			return
		}

		// контекст для ожидания запуска
		waitStartCtx, cancelWaitStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitStart()

		// ждём запуска с контекстом и экспоненциальной задержкой
		if waitErr := h.waitForServiceStatus(waitStartCtx, client, service.ServiceName, utils.ServiceRunning); waitErr != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не запустилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешного запуска
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` перезапущена", service.DisplayedName))

	case utils.ServiceStopped:
		// просто запускаем

		// контекст для запуска
		startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStart()

		if stdout, err = client.RunCommand(startCtx, startCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось запустить службу `%s`", service.DisplayedName))
			return
		}

		// проверяем вывод на FAILED, .т.е на ошибку запуска службы
		// (если exit code != 0, то в выводе точно будет FAILED)
		if serviceErr := errs.ParseServiceError(stdout); serviceErr != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID), logger.String("err", serviceErr.Error()))
			response.ErrorJSON(w, http.StatusBadRequest, serviceErr.Error())
			return
		}

		// контекст для ожидания запуска
		waitStartCtx, cancelWaitStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWaitStart()

		// ждём запуска с контекстом и экспоненциальной задержкой
		if waitErr := h.waitForServiceStatus(waitStartCtx, client, service.ServiceName, utils.ServiceRunning); waitErr != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не запустилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, creds.ServerID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` перезапущена", service.DisplayedName))

	case utils.ServiceStartPending, utils.ServiceStopPending:
		// уже в процессе
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже изменяет состояние на сервере `%s`, id=%d",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже изменяет состояние, попробуйте позже", service.DisplayedName))

	default:
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем перезапуск",
			service.DisplayedName, creds.ServiceID, server.Name, creds.ServerID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем перезапуск", service.DisplayedName))
	}
}

// Вспомогательный метод для ожидания статуса
func (h *ControlHandler) waitForServiceStatus(ctx context.Context, client service_control.Client, serviceName string, expectedStatus int) error {
	statusCmd := fmt.Sprintf("sc query \"%s\"", serviceName)
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			result, err := client.RunCommand(ctx, statusCmd)
			if err != nil {
				return err
			}

			currentStatus := utils.GetStatus(result)

			if currentStatus == expectedStatus {
				return nil
			}

			// Если в переходном состоянии - ждём дальше
			if currentStatus == utils.ServiceStartPending || currentStatus == utils.ServiceStopPending {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			// Неожиданное состояние
			return fmt.Errorf("неожиданное состояние: %d, ожидалось: %d", currentStatus, expectedStatus)
		}
	}
}
