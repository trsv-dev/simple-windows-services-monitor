package control_handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	netutilsMock "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// createContextWithCreds Создаёт контекст с учётными данными пользователя.
func createContextWithCreds(login string, userID, serverID, serviceID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextkeys.Login, login)
	ctx = context.WithValue(ctx, contextkeys.ID, userID)
	ctx = context.WithValue(ctx, contextkeys.ServerID, serverID)
	ctx = context.WithValue(ctx, contextkeys.ServiceID, serviceID)
	return ctx
}

// ============================================================================
// ServiceStop
// ============================================================================

// TestServiceStopErrServerNotFound Проверяет обработку ErrServerNotFound.
func TestServiceStopErrServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	// возвращаем ErrServerNotFound
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("server not in database")))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	// создаём запрос с контекстом пользователя
	ctx := createContextWithCreds("user", 1, 100, 10)
	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем, что вернулась 404
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер не найден", got.Message)
}

// TestServiceStopGetServerGenericError Проверяет обработку generic ошибки при GetServerWithPassword.
func TestServiceStopGetServerGenericError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	// generic ошибка (не специфичная ErrServerNotFound)
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(nil, errors.New("database connection timeout"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	ctx := createContextWithCreds("user", 1, 100, 10)
	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о сервере", got.Message)
}

// TestServiceStopErrServiceNotFound Проверяет обработку ErrServiceNotFound.
func TestServiceStopErrServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// возвращаем ErrServiceNotFound
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(nil, errs.NewErrServiceNotFound(1, 100, 10, errors.New("service not found")))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	ctx := createContextWithCreds("user", 1, 100, 10)
	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба не найдена", got.Message)
}

// TestServiceStopGetServiceGenericError Проверяет обработку generic ошибки при GetService.
func TestServiceStopGetServiceGenericError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// generic ошибка (не специфичная ErrServiceNotFound)
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(nil, errors.New("database read error"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	ctx := createContextWithCreds("user", 1, 100, 10)
	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о службе", got.Message)
}

// TestServiceStopCheckWinRMFalse Проверяет обработку недоступного хоста.
func TestServiceStopCheckWinRMFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост недоступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(false)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер недоступен", got.Message)
}

// TestServiceStopCreateClientError Проверяет обработку ошибки создания WinRM клиента.
func TestServiceStopCreateClientError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// ошибка создания клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(nil, errors.New("WinRM authentication failed"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка подключения к серверу", got.Message)
}

// TestServiceStopQueryStatusError Проверяет ошибку при получении статуса.
func TestServiceStopQueryStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// ошибка при sc query
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("", errors.New("WinRM connection timeout"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось получить статус службы `Test Service`", got.Message)
}

// TestServiceStopRunningSuccess Проверяет успешную остановку из RUNNING.
func TestServiceStopRunningSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность вызовов:
	// 1. sc query (начальный статус) - возвращает RUNNING
	// 2. sc stop - успешно, пустой вывод
	// 3. sc query (в waitForServiceStatus) - возвращает STOPPED
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
	)

	// обновляем статус в БД
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` остановлена", got.Message)
}

// TestServiceStopAlreadyStopped Проверяет обработку уже остановленной службы.
func TestServiceStopAlreadyStopped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// sc query возвращает STOPPED
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 1 STOPPED", nil)

	// обновляем статус в БД
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже остановлена", got.Message)
}

// TestServiceStopRunCommandError Проверяет ошибку при sc stop команде.
func TestServiceStopRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - успешно, RUNNING
	// 2. sc stop - ошибка транспорта
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", errors.New("WinRM transport error")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось остановить службу", got.Message)
}

// TestServiceStopParseServiceError Проверяет парсинг ошибки из sc stop вывода.
func TestServiceStopParseServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query - RUNNING
	// 2. sc stop - возвращает FAILED 1061
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("[SC] ControlService FAILED 1061:\n\nThe service cannot accept control messages at this time.", nil),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	// Проверяем, что ошибка парсится корректно (код 1061)
	assert.NotEmpty(t, got.Message)
}

// TestServiceStopWaitForStatusError Проверяет ошибку при ожидании статуса.
func TestServiceStopWaitForStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - RUNNING
	// 2. sc stop - успешно
	// 3. sc query (в waitForServiceStatus) - ошибка
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("", errors.New("WinRM connection lost")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не остановилась в ожидаемое время", got.Message)
}

// ============================================================================
// ServiceStart
// ============================================================================

// TestServiceStartRunningSuccess Проверяет успешный запуск из STOPPED.
func TestServiceStartRunningSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - STOPPED
	// 2. sc start - успешно
	// 3. sc query (в waitForServiceStatus) - RUNNING
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
	)

	// обновляем статус в БД
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` запущена", got.Message)
}

// TestServiceStartAlreadyRunning Проверяет обработку уже запущенной службы.
func TestServiceStartAlreadyRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// sc query возвращает RUNNING
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil)

	// обновляем статус в БД
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже запущена", got.Message)
}

// TestServiceStartRunCommandError Проверяет ошибку при sc start команде.
func TestServiceStartRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - STOPPED
	// 2. sc start - ошибка транспорта
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", errors.New("WinRM transport error")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось запустить службу", got.Message)
}

// TestServiceStartParseServiceError Проверяет парсинг ошибки из sc start вывода.
func TestServiceStartParseServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query - STOPPED
	// 2. sc start - возвращает FAILED 1051 (зависимые службы)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("[SC] ControlService FAILED 1051:\n\nA stop control has been sent to a service that other running services are dependent on.", nil),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.NotEmpty(t, got.Message)
}

// TestServiceStartWaitForStatusError Проверяет ошибку при ожидании статуса.
func TestServiceStartWaitForStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - STOPPED
	// 2. sc start - успешно
	// 3. sc query (в waitForServiceStatus) - ошибка
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("", errors.New("WinRM connection lost")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не запустилась в ожидаемое время", got.Message)
}

// ============================================================================
// ServiceRestart
// ============================================================================

// TestServiceRestartRunningSuccess Проверяет успешный перезапуск из RUNNING.
func TestServiceRestartRunningSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - RUNNING
	// 2. sc stop - успешно
	// 3. sc query (ожидание остановки) - STOPPED
	// 4. sc start - успешно
	// 5. sc query (ожидание запуска) - RUNNING
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
	)

	// обновляем статус в БД дважды (остановка и запуск)
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(nil)

	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` перезапущена", got.Message)
}

// TestServiceRestartStoppedSuccess Проверяет перезапуск из STOPPED.
func TestServiceRestartStoppedSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - STOPPED
	// 2. sc start - успешно
	// 3. sc query (ожидание запуска) - RUNNING
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
	)

	// обновляем статус в БД
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` перезапущена", got.Message)
}

// TestServiceRestartRunCommandStopError Проверяет ошибку при sc stop (для RUNNING).
func TestServiceRestartRunCommandStopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - RUNNING
	// 2. sc stop - ошибка транспорта
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", errors.New("WinRM connection error")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось остановить службу `Test Service`", got.Message)
}

// TestServiceRestartParseServiceErrorStop Проверяет парсинг ошибки из sc stop вывода.
func TestServiceRestartParseServiceErrorStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query - RUNNING
	// 2. sc stop - возвращает FAILED 1061
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("[SC] ControlService FAILED 1061:\n\nThe service cannot accept control messages at this time.", nil),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.NotEmpty(t, got.Message)
}

// TestServiceRestartWaitForStopError Проверяет ошибку при ожидании остановки.
func TestServiceRestartWaitForStopError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - RUNNING
	// 2. sc stop - успешно
	// 3. sc query (ожидание остановки) - ошибка
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("", errors.New("WinRM connection lost")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не остановилась в ожидаемое время", got.Message)
}

// TestServiceRestartRunCommandStartError Проверяет ошибку запуска после остановки.
func TestServiceRestartRunCommandStartError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - RUNNING
	// 2. sc stop - успешно
	// 3. sc query (ожидание остановки) - STOPPED
	// 4. sc start - ошибка транспорта
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", errors.New("service startup failed")),
	)

	// БД будет обновлён для остановки
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось запустить службу `Test Service`", got.Message)
}

// TestServiceRestartParseServiceErrorStart Проверяет парсинг ошибки из sc start вывода.
func TestServiceRestartParseServiceErrorStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query - STOPPED
	// 2. sc start - возвращает FAILED 1052
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("[SC] ControlService FAILED 1052:\n\nThe requested control is not valid for this service.", nil),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.NotEmpty(t, got.Message)
}

// TestServiceRestartWaitForStartError Проверяет ошибку при ожидании запуска.
func TestServiceRestartWaitForStartError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	ctx := createContextWithCreds("user", 1, 100, 10)

	// получаем сервер успешно
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
		Return(&models.Server{
			ID:       100,
			Name:     "TestServer",
			Address:  "192.168.1.1",
			Username: "admin",
			Password: "password",
		}, nil)

	// получаем службу успешно
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(100), int64(10), int64(1)).
		Return(&models.Service{
			ID:            10,
			ServiceName:   "TestService",
			DisplayedName: "Test Service",
		}, nil)

	// хост доступен
	mockChecker.EXPECT().
		CheckWinRM(ctx, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// Последовательность:
	// 1. sc query (начальный статус) - STOPPED
	// 2. sc start - успешно
	// 3. sc query (ожидание запуска) - ошибка
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("", errors.New("WinRM connection lost")),
	)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не запустилась в ожидаемое время", got.Message)
}
