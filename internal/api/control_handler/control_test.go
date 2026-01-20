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

// TestServiceStopCreateClientError Проверяет обработку ошибки при создании WinRM клиента.
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

	// ошибка при создании WinRM клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(nil, errors.New("authentication failed"))

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

// TestServiceStopRunCommandStatusError Проверяет обработку ошибки команды sc query.
func TestServiceStopRunCommandStatusError(t *testing.T) {
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

	// ошибка при выполнении sc query команды
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

// TestServiceStopRunCommandStopError Проверяет обработку ошибки команды sc stop.
func TestServiceStopRunCommandStopError(t *testing.T) {
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

	// последовательность: сначала sc query (RUNNING), потом sc stop (ошибка)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", errors.New("access denied")),
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

	// последовательность: sc query (RUNNING), потом sc stop (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
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

// TestServiceStopStoppedAlready Проверяет обработку уже остановленной службы.
func TestServiceStopStoppedAlready(t *testing.T) {
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

	// статус: STOPPED (уже остановлена)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 1 STOPPED", nil)

	// обновляем статус в БД для синхронизации
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

// TestServiceStopStopPendingConflict Проверяет конфликт для STOP_PENDING.
func TestServiceStopStopPendingConflict(t *testing.T) {
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

	// статус: STOP_PENDING (уже в процессе остановки)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 3 STOP_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже останавливается", got.Message)
}

// TestServiceStopPausePendingConflict Проверяет конфликт для PAUSE_PENDING.
func TestServiceStopPausePendingConflict(t *testing.T) {
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

	// статус: PAUSE_PENDING (в процессе паузы)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 6 PAUSE_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже останавливается", got.Message)
}

// TestServiceStopPausedDefaultError Проверяет неожиданный статус в default case.
func TestServiceStopPausedDefaultError(t *testing.T) {
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

	// статус: PAUSED (код 7) - неожиданный, попадает в default
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 7 PAUSED", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` находится в состоянии, не позволяющем остановку", got.Message)
}

// TestServiceStopStartPendingSuccess Проверяет успешную остановку из START_PENDING.
func TestServiceStopStartPendingSuccess(t *testing.T) {
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

	// последовательность: sc query (START_PENDING), потом sc stop (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 2 START_PENDING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
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

// TestServiceStopDbErrorAfterStop Проверяет обработку ошибки БД после остановки.
func TestServiceStopDbErrorAfterStop(t *testing.T) {
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

	// последовательность: sc query (RUNNING), потом sc stop (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", nil),
	)

	// ошибка БД при обновлении статуса, но служба реально остановлена
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(errors.New("database write error"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех так как служба реально остановлена
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` остановлена", got.Message)
}

// TestServiceStopDbErrorWhenStopped Проверяет обработку ошибки БД для остановленной службы.
func TestServiceStopDbErrorWhenStopped(t *testing.T) {
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

	// статус: уже STOPPED
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 1 STOPPED", nil)

	// ошибка БД при синхронизации статуса, но служба реально остановлена
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(errors.New("database connection lost"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStop(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже остановлена", got.Message)
}

// TestNewControlHandler Проверяет конструктор и инициализацию ControlHandler.
func TestNewControlHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockWinRMPort := "5985"

	// создаём handler через конструктор
	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	// проверяем инициализацию всех полей
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.storage)
	assert.NotNil(t, handler.clientFactory)
	assert.NotNil(t, handler.checker)
}

// ============================================================================
// ServiceStart
// ============================================================================

// TestServiceStartErrServerNotFound Проверяет обработку ErrServerNotFound.
func TestServiceStartErrServerNotFound(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем, что вернулась 404
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер не найден", got.Message)
}

// TestServiceStartGetServerGenericError Проверяет обработку generic ошибки при GetServerWithPassword.
func TestServiceStartGetServerGenericError(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о сервере", got.Message)
}

// TestServiceStartErrServiceNotFound Проверяет обработку ErrServiceNotFound.
func TestServiceStartErrServiceNotFound(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба не найдена", got.Message)
}

// TestServiceStartGetServiceGenericError Проверяет обработку generic ошибки при GetService.
func TestServiceStartGetServiceGenericError(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о службе", got.Message)
}

// TestServiceStartCheckWinRMFalse Проверяет обработку недоступного хоста.
func TestServiceStartCheckWinRMFalse(t *testing.T) {
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

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер недоступен", got.Message)
}

// TestServiceStartCreateClientError Проверяет обработку ошибки при создании WinRM клиента.
func TestServiceStartCreateClientError(t *testing.T) {
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

	// ошибка при создании WinRM клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(nil, errors.New("authentication failed"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка подключения к серверу", got.Message)
}

// TestServiceStartRunCommandStatusError Проверяет обработку ошибки команды sc query.
func TestServiceStartRunCommandStatusError(t *testing.T) {
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

	// ошибка при выполнении sc query команды
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("", errors.New("WinRM connection timeout"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось получить статус службы `Test Service`", got.Message)
}

// TestServiceStartRunCommandStartError Проверяет обработку ошибки команды sc start.
func TestServiceStartRunCommandStartError(t *testing.T) {
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

	// последовательность: сначала sc query (STOPPED), потом sc start (ошибка)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", errors.New("access denied")),
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

// TestServiceStartStoppedSuccess Проверяет успешный запуск из STOPPED.
func TestServiceStartStoppedSuccess(t *testing.T) {
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

	// последовательность: sc query (STOPPED), потом sc start (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
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

// TestServiceStartStopPendingSuccess Проверяет успешный запуск из STOP_PENDING.
func TestServiceStartStopPendingSuccess(t *testing.T) {
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

	// последовательность: sc query (STOP_PENDING), потом sc start (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 3 STOP_PENDING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
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

// TestServiceStartRunningAlready Проверяет обработку уже запущенной службы.
func TestServiceStartRunningAlready(t *testing.T) {
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

	// статус: RUNNING (уже запущена)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil)

	// обновляем статус в БД для синхронизации
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

// TestServiceStartStartPendingConflict Проверяет конфликт для START_PENDING.
func TestServiceStartStartPendingConflict(t *testing.T) {
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

	// статус: START_PENDING (уже в процессе запуска)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 2 START_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже запускается", got.Message)
}

// TestServiceStartPausePendingConflict Проверяет конфликт для PAUSE_PENDING.
func TestServiceStartPausePendingConflict(t *testing.T) {
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

	// статус: PAUSE_PENDING (в процессе паузы)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 6 PAUSE_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже запускается", got.Message)
}

// TestServiceStartPausedDefaultError Проверяет неожиданный статус в default case.
func TestServiceStartPausedDefaultError(t *testing.T) {
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

	// статус: PAUSED (код 7) - неожиданный, попадает в default
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 7 PAUSED", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` находится в состоянии, не позволяющем запуск", got.Message)
}

// TestServiceStartDbErrorAfterStart Проверяет обработку ошибки БД после запуска.
func TestServiceStartDbErrorAfterStart(t *testing.T) {
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

	// последовательность: sc query (STOPPED), потом sc start (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
	)

	// ошибка БД при обновлении статуса, но служба реально запущена
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(errors.New("database write error"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех так как служба реально запущена
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` запущена", got.Message)
}

// TestServiceStartDbErrorWhenRunning Проверяет обработку ошибки БД для запущенной службы.
func TestServiceStartDbErrorWhenRunning(t *testing.T) {
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

	// статус: уже RUNNING
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil)

	// ошибка БД при синхронизации статуса, но служба реально запущена
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(errors.New("database connection lost"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceStart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже запущена", got.Message)
}

// ============================================================================
// ServiceRestart
// ============================================================================

// TestServiceRestartErrServerNotFound Проверяет обработку ErrServerNotFound.
func TestServiceRestartErrServerNotFound(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем, что вернулась 404
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер не найден", got.Message)
}

// TestServiceRestartGetServerGenericError Проверяет обработку generic ошибки при GetServerWithPassword.
func TestServiceRestartGetServerGenericError(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о сервере", got.Message)
}

// TestServiceRestartErrServiceNotFound Проверяет обработку ErrServiceNotFound.
func TestServiceRestartErrServiceNotFound(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба не найдена", got.Message)
}

// TestServiceRestartGetServiceGenericError Проверяет обработку generic ошибки при GetService.
func TestServiceRestartGetServiceGenericError(t *testing.T) {
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
	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка при получении информации о службе", got.Message)
}

// TestServiceRestartCheckWinRMFalse Проверяет обработку недоступного хоста.
func TestServiceRestartCheckWinRMFalse(t *testing.T) {
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

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// проверяем 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Сервер недоступен", got.Message)
}

// TestServiceRestartCreateClientError Проверяет обработку ошибки при создании WinRM клиента.
func TestServiceRestartCreateClientError(t *testing.T) {
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

	// ошибка при создании WinRM клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(nil, errors.New("authentication failed"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Ошибка подключения к серверу", got.Message)
}

// TestServiceRestartRunCommandStatusError Проверяет обработку ошибки команды sc query.
func TestServiceRestartRunCommandStatusError(t *testing.T) {
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

	// ошибка при выполнении sc query команды
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("", errors.New("WinRM connection timeout"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось получить статус службы `Test Service`", got.Message)
}

// TestServiceRestartRunCommandStopError Проверяет обработку ошибки команды sc stop.
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

	// последовательность: сначала sc query (RUNNING), потом sc stop (ошибка)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 4 RUNNING", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc stop "TestService"`).
			Return("", errors.New("access denied")),
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

// TestServiceRestartWaitForStatusError Проверяет ошибку ожидания остановки.
func TestServiceRestartWaitForStatusError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockWinRMPort := "5985"

	// используем контекст с КОРОТКИМ таймаутом
	ctx := createContextWithCreds("user", 1, 100, 10)
	// добавляем таймаут 100ms для ожидания остановки, чтобы тест выполнялся быстро
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

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
		CheckWinRM(ctxWithTimeout, "192.168.1.1", mockWinRMPort, time.Duration(0)).
		Return(true)

	// клиент создан успешно
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.1", "admin", "password").
		Return(mockClient, nil)

	// первая sc query - проверка начального статуса (RUNNING)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil).
		Times(1)

	// sc stop - остановка
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc stop "TestService"`).
		Return("", nil).
		Times(1)

	// повторные sc query при ожидании остановки - служба остаётся RUNNING
	// вызовется 1-3 раза за 100ms таймаута
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil).
		MinTimes(1).
		MaxTimes(3)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctxWithTimeout)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// таймаут при ожидании остановки
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не остановилась в ожидаемое время", got.Message)
}

// TestServiceRestartWaitForStatusRunCommandError Проверяет ошибку sc query внутри цикла ожидания статуса.
func TestServiceRestartWaitForStatusRunCommandError(t *testing.T) {
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

	// первая sc query - проверка начального статуса (RUNNING)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil).
		Times(1)

	// sc stop - остановка
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc stop "TestService"`).
		Return("", nil).
		Times(1)

	// вторая sc query вернёт ошибку во время ожидания остановки
	// это вызовет return fmt.Errorf("ошибка получения статуса службы: %w", err)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("", errors.New("WinRM connection lost")).
		Times(1)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ошибка при ожидании остановки (ошибка sc query)
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` не остановилась в ожидаемое время", got.Message)
}

// TestServiceRestartRunCommandStartErrorAfterStop Проверяет ошибку запуска после остановки (для RUNNING → остановка → запуск ошибка).
func TestServiceRestartRunCommandStartErrorAfterStop(t *testing.T) {
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

	// последовательность: sc query (RUNNING), sc stop (успешно), ожидание остановки (успешно), sc start (ошибка)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 4 RUNNING", nil).
		Times(1)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc stop "TestService"`).
		Return("", nil).
		Times(1)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 1 STOPPED", nil).
		Times(1)

	// ошибка при запуске
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc start "TestService"`).
		Return("", errors.New("service startup failed")).
		Times(1)

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

	// ошибка запуска
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Не удалось запустить службу `Test Service`", got.Message)
}

// TestServiceRestartRunCommandStartErrorFromStopped Проверяет ошибку запуска для STOPPED.
func TestServiceRestartRunCommandStartErrorFromStopped(t *testing.T) {
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

	// последовательность: sc query (STOPPED), sc start (ошибка)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 1 STOPPED", nil).
		Times(1)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc start "TestService"`).
		Return("", errors.New("access denied")).
		Times(1)

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

// TestServiceRestartRunCommandStartError Проверяет обработку ошибки команды sc start.
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

	// последовательность: sc query (RUNNING), sc stop (успешно), ожидание остановки (успешно), sc start (ошибка)
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

	// последовательность: sc query (RUNNING), sc stop (успешно), ожидание остановки (успешно), sc start (успешно)
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
	)

	// обновляем статус в БД - сначала остановлена, потом работает
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

// TestServiceRestartStoppedSuccess Проверяет успешный перезапуск из STOPPED.
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

	// последовательность: sc query (STOPPED), sc start (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
	)

	// обновляем статус в БД для синхронизации
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

// TestServiceRestartStartPendingConflict Проверяет конфликт для START_PENDING.
func TestServiceRestartStartPendingConflict(t *testing.T) {
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

	// статус: START_PENDING (уже в процессе изменения состояния)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 2 START_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже изменяет состояние, попробуйте позже", got.Message)
}

// TestServiceRestartStopPendingConflict Проверяет конфликт для STOP_PENDING.
func TestServiceRestartStopPendingConflict(t *testing.T) {
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

	// статус: STOP_PENDING (уже в процессе изменения состояния)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 3 STOP_PENDING", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// ожидаем конфликт 409
	assert.Equal(t, http.StatusConflict, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` уже изменяет состояние, попробуйте позже", got.Message)
}

// TestServiceRestartPausedDefaultError Проверяет неожиданный статус в default case.
func TestServiceRestartPausedDefaultError(t *testing.T) {
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

	// статус: PAUSED (код 7) - неожиданный, попадает в default
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "TestService"`).
		Return("STATE : 7 PAUSED", nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	var got response.APIError
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` находится в состоянии, не позволяющем перезапуск", got.Message)
}

// TestServiceRestartDbErrorAfterStop Проверяет обработку ошибки БД после остановки.
func TestServiceRestartDbErrorAfterStop(t *testing.T) {
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

	// последовательность: sc query (RUNNING), sc stop (успешно), ожидание остановки (успешно), sc start (успешно)
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
	)

	// ошибка БД при обновлении статуса после остановки, но продолжаем
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
		Return(errors.New("database write error"))
	// успешное обновление после запуска
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(nil)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех так как служба реально перезапущена
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` перезапущена", got.Message)
}

// TestServiceRestartDbErrorAfterStart Проверяет обработку ошибки БД после запуска.
func TestServiceRestartDbErrorAfterStart(t *testing.T) {
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

	// последовательность: sc query (STOPPED), sc start (успешно)
	gomock.InOrder(
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc query "TestService"`).
			Return("STATE : 1 STOPPED", nil),
		mockClient.EXPECT().
			RunCommand(gomock.Any(), `sc start "TestService"`).
			Return("", nil),
	)

	// ошибка БД при обновлении статуса после запуска, но служба реально запущена
	mockStorage.EXPECT().
		ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
		Return(errors.New("database connection lost"))

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker, mockWinRMPort)

	r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServiceRestart(w, r)

	res := w.Result()
	defer res.Body.Close()

	// несмотря на ошибку БД, возвращаем успех
	assert.Equal(t, http.StatusOK, res.StatusCode)
	var got response.APISuccess
	json.NewDecoder(res.Body).Decode(&got)
	assert.Equal(t, "Служба `Test Service` перезапущена", got.Message)
}
