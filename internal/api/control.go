package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/utils"
)

// ServiceStop Остановка службы.
func (h *AppHandler) ServiceStop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)
	serviceID := ctx.Value(contextkeys.ServiceID).(int)

	// получаем сервер
	server, err := h.storage.GetServer(ctx, serverID, login)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден", logger.String("login", ErrServerNotFound.Login),
				logger.Int("serverID", ErrServerNotFound.ID), logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	// получаем службу
	service, err := h.storage.GetService(ctx, serverID, serviceID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
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

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	statusCmd := fmt.Sprintf("sc query %s", service.ServiceName)
	stopCmd := fmt.Sprintf("sc stop %s", service.ServiceName)

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))

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

		if _, err = client.RunCommand(stopCtx, stopCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Не удалось остановить службу")
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешной остановки
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
			// не возвращаем ошибку пользователю, т.к. служба реально остановлена
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` остановлена", service.DisplayedName))

	case utils.ServiceStopped:
		// уже остановлена

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d уже остановлена",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` уже остановлена", service.DisplayedName))

	case utils.ServiceStopPending, utils.ServicePausePending:
		// уже выполняется остановка/пауза
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже останавливается на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже останавливается", service.DisplayedName))

	default:
		// неожиданный статус
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем остановку",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем остановку", service.DisplayedName))

	}
}

// ServiceStart Запуск службы.
func (h *AppHandler) ServiceStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)
	serviceID := ctx.Value(contextkeys.ServiceID).(int)

	// получаем сервер
	server, err := h.storage.GetServer(ctx, serverID, login)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден", logger.String("login", ErrServerNotFound.Login),
				logger.Int("serverID", ErrServerNotFound.ID), logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	// получаем службу
	service, err := h.storage.GetService(ctx, serverID, serviceID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
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
	startCmd := fmt.Sprintf("sc start %s", service.ServiceName)

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))

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

		if _, err = client.RunCommand(startCtx, startCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Не удалось запустить службу")
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешного запуска
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` запущена", service.DisplayedName))

	case utils.ServiceRunning:
		// уже запущена

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d уже запущена",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` уже запущена", service.DisplayedName))

	case utils.ServiceStartPending, utils.ServicePausePending:
		// уже выполняется запуск/пауза
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже запускается на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже запускается", service.DisplayedName))

	default:
		// неожиданный статус
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем запуск",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем запуск", service.DisplayedName))
	}
}

// ServiceRestart Перезапуск службы.
func (h *AppHandler) ServiceRestart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	login := ctx.Value(contextkeys.Login).(string)
	serverID := ctx.Value(contextkeys.ServerID).(int)
	serviceID := ctx.Value(contextkeys.ServiceID).(int)

	// получаем сервер
	server, err := h.storage.GetServer(ctx, serverID, login)

	var ErrServerNotFound *errs.ErrServerNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServerNotFound):
			logger.Log.Warn("Сервер не найден", logger.String("login", ErrServerNotFound.Login),
				logger.Int("serverID", ErrServerNotFound.ID), logger.String("err", ErrServerNotFound.Err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Сервер не найден")
			return
		default:
			logger.Log.Warn("Ошибка при получении информации о сервере", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о сервере")
			return
		}
	}

	// получаем службу
	service, err := h.storage.GetService(ctx, serverID, serviceID, login)

	var ErrServiceNotFound *errs.ErrServiceNotFound

	if err != nil {
		switch {
		case errors.As(err, &ErrServiceNotFound):
			logger.Log.Warn("Служба не найдена", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusNotFound, "Служба не найдена")
			return
		default:
			logger.Log.Error("Ошибка при получении информации о службе", logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError, "Ошибка при получении информации о службе")
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
	stopCmd := fmt.Sprintf("sc stop %s", service.ServiceName)
	startCmd := fmt.Sprintf("sc start %s", service.ServiceName)

	// контекст для получения статуса
	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := client.RunCommand(statusCtx, statusCmd)
	if err != nil {
		logger.Log.Warn(fmt.Sprintf("Не удалось получить статус службы `%s`, id=%d на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))

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

		if _, err = client.RunCommand(stopCtx, stopCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось остановить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось остановить службу `%s`", service.DisplayedName))
			return
		}

		// контекст для ожидания остановки
		waitCtx, cancelWait := context.WithTimeout(ctx, 30*time.Second)
		defer cancelWait()

		// ждём остановки с контекстом и экспоненциальной задержкой
		if err := h.waitForServiceStatus(waitCtx, client, service.ServiceName, utils.ServiceStopped); err != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Служба `%s` не остановилась в ожидаемое время", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешной остановки
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Остановлена"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		// теперь запускаем

		// контекст для запуска
		startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStart()

		if _, err = client.RunCommand(startCtx, startCmd); err != nil {
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось запустить службу `%s`", service.DisplayedName))
			return
		}

		// обновляем статус службы в БД для всех пользователей после успешного запуска
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` перезапущена", service.DisplayedName))

	case utils.ServiceStopped:
		// просто запускаем

		// контекст для запуска
		startCtx, cancelStart := context.WithTimeout(ctx, 30*time.Second)
		defer cancelStart()

		if _, err = client.RunCommand(startCtx, startCmd); err != nil {
			logger.Log.Warn(fmt.Sprintf("Не удалось запустить службу `%s`, id=%d на сервере `%s`, id=%d",
				service.DisplayedName, serviceID, server.Name, serverID), logger.String("err", err.Error()))
			response.ErrorJSON(w, http.StatusInternalServerError,
				fmt.Sprintf("Не удалось запустить службу `%s`", service.DisplayedName))
			return
		}

		// обновляем статус в БД на всякий случай для синхронизации
		if err = h.storage.ChangeServiceStatus(ctx, serverID, service.ServiceName, "Работает"); err != nil {
			logger.Log.Error("Не удалось обновить статус службы в БД", logger.String("err", err.Error()))
		}

		response.SuccessJSON(w, http.StatusOK,
			fmt.Sprintf("Служба `%s` перезапущена", service.DisplayedName))

	case utils.ServiceStartPending, utils.ServiceStopPending:
		// уже в процессе
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d уже изменяет состояние на сервере `%s`, id=%d",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusConflict,
			fmt.Sprintf("Служба `%s` уже изменяет состояние, попробуйте позже", service.DisplayedName))

	default:
		logger.Log.Warn(fmt.Sprintf("Служба `%s`, id=%d на сервере `%s`, id=%d находится в состоянии, не позволяющем перезапуск",
			service.DisplayedName, serviceID, server.Name, serverID))
		response.ErrorJSON(w, http.StatusBadRequest,
			fmt.Sprintf("Служба `%s` находится в состоянии, не позволяющем перезапуск", service.DisplayedName))
	}
}

// Вспомогательный метод для ожидания статуса
func (h *AppHandler) waitForServiceStatus(ctx context.Context, client *service_control.WinRMClient, serviceName string, expectedStatus int) error {
	statusCmd := fmt.Sprintf("sc query %s", serviceName)

	// Экспоненциальная задержка: 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s
	backoff := 100 * time.Millisecond
	maxBackoff := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err() // таймаут или отмена клиентом
		case <-time.After(backoff):
			result, err := client.RunCommand(ctx, statusCmd)
			if err != nil {
				return fmt.Errorf("ошибка получения статуса службы: %w", err)
			}

			if utils.GetStatus(result) == expectedStatus {
				return nil // статус успешно получен
			}

			// увеличиваем задержку экспоненциально
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
