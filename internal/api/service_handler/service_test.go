package service_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	netutilsMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
	workerMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/worker/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewServiceHandler Проверяет создание ServiceHandler.
func TestNewServiceHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	// проверяем что handler создан
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.storage)
	assert.NotNil(t, handler.clientFactory)
	assert.NotNil(t, handler.checker)
}

// TestListOfServicesSuccess Проверяет успешное получение списка всех служб.
func TestListOfServicesSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// сервер возвращает список служб
	servicesOutput := "Service1\nService2\nService3"
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `powershell -Command "Get-Service | Select-Object -ExpandProperty Name"`).
		Return(servicesOutput, nil)

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем ответ
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)
	assert.NotNil(t, response["services"])

	services := response["services"].([]interface{})
	assert.Equal(t, 3, len(services))
	assert.Equal(t, "Service1", services[0])
	assert.Equal(t, "Service2", services[1])
	assert.Equal(t, "Service3", services[2])
}

// TestListOfServicesServerNotFound Проверяет ошибку когда сервер не найден.
func TestListOfServicesServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, &errs.ErrServerNotFound{
			UserID:   1,
			ServerID: 1,
			Err:      errors.New("server not found"),
		})

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Сервер не найден")
}

// TestListOfServicesDatabaseError Проверяет ошибку БД при получении сервера.
func TestListOfServicesDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, errors.New("database error"))

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка при получении информации о сервере")
}

// TestListOfServicesServerUnreachable Проверяет когда сервер недоступен.
func TestListOfServicesServerUnreachable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(false)

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Contains(t, w.Body.String(), "недоступен")
}

// TestListOfServicesClientFactoryError Проверяет ошибку создания WinRM клиента.
func TestListOfServicesClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	// ошибка создания клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(nil, errors.New("connection error"))

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка подключения к серверу")
}

// TestListOfServicesRunCommandError Проверяет ошибку выполнения команды.
func TestListOfServicesRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// ошибка выполнения команды
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `powershell -Command "Get-Service | Select-Object -ExpandProperty Name"`).
		Return("", errors.New("command failed"))

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Не удалось получить список служб")
}

// TestListOfServicesEmptyList Проверяет пустой список служб.
func TestListOfServicesEmptyList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// сервер возвращает пустой результат
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `powershell -Command "Get-Service | Select-Object -ExpandProperty Name"`).
		Return("", nil)

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем пустой список
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)

	services := response["services"].([]interface{})
	assert.Equal(t, 0, len(services))
}

// TestListOfServicesWhitespaceHandling Проверяет обработку пробелов в названиях служб.
func TestListOfServicesWhitespaceHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// сервер возвращает список с пробелами и пустыми строками
	servicesOutput := "  Service1  \n\n  Service2  \n\nService3"
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `powershell -Command "Get-Service | Select-Object -ExpandProperty Name"`).
		Return(servicesOutput, nil)

	r := httptest.NewRequest(http.MethodGet, "/services/available", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.ListOfServices(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем что пробелы и пустые строки удалены
	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	assert.NoError(t, err)

	services := response["services"].([]interface{})
	assert.Equal(t, 3, len(services))
	assert.Equal(t, "Service1", services[0])
	assert.Equal(t, "Service2", services[1])
	assert.Equal(t, "Service3", services[2])
}

// TestAddServiceInvalidJSON Проверяет добавление службы с невалидным JSON.
func TestAddServiceInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	// невалидный JSON
	body := []byte(`{invalid json}`)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "формат")
}

// TestAddServiceInvalidServiceData Проверяет валидацию данных службы.
func TestAddServiceInvalidServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	tests := []struct {
		name    string
		service models.Service
	}{
		{
			name: "пустое имя службы",
			service: models.Service{
				ServiceName:   "",
				DisplayedName: "Test",
			},
		},
		{
			name: "пустое отображаемое имя",
			service: models.Service{
				ServiceName:   "test",
				DisplayedName: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.service)
			r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
			w := httptest.NewRecorder()

			ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
			ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
			ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
			r = r.WithContext(ctx)

			handler.AddService(w, r)

			// проверяем ошибку валидации
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
}

// TestAddServiceServerNotFound Проверяет добавление когда сервер не найден.
func TestAddServiceServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, &errs.ErrServerNotFound{
			UserID:   1,
			ServerID: 1,
			Err:      errors.New("server not found"),
		})

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Сервер не найден")
}

// TestAddServiceDatabaseError Проверяет ошибку БД при получении сервера.
func TestAddServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, errors.New("database error"))

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка при получении информации о сервере")
}

// TestAddServiceServerUnreachable Проверяет добавление на недоступный сервер.
func TestAddServiceServerUnreachable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(false)

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "недоступен")
}

// TestAddServiceClientFactoryError Проверяет ошибку создания WinRM клиента.
func TestAddServiceClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	// ошибка создания клиента
	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(nil, errors.New("connection error"))

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка подключения к серверу")
}

// TestAddServiceRunCommandError Проверяет ошибку выполнения команды.
func TestAddServiceRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// ошибка выполнения команды
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("", errors.New("command failed"))

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Не удалось получить статус службы")
}

// TestAddServiceNotExistsOnServer Проверяет что служба не существует на сервере.
func TestAddServiceNotExistsOnServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// команда возвращает код 1060 (служба не найдена)
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("QueryServiceConfig FAILED 1060", nil)

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "не найдена на сервере")
}

// TestAddServiceDuplicateService Проверяет добавление дублирующейся службы.
func TestAddServiceDuplicateService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	// команда возвращает успешный результат
	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("SERVICE_NAME: testservice\nSTATE: 4 RUNNING", nil)

	// ошибка дублирования в БД
	mockStorage.EXPECT().
		AddService(gomock.Any(), int64(1), int64(1), gomock.Any()).
		Return(nil, &errs.ErrDuplicatedService{
			ServiceName: "testservice",
			Err:         errors.New("duplicate"),
		})

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "уже была добавлена")
}

// TestAddServiceServerNotFoundInAddService Проверяет ошибку сервера при AddService.
func TestAddServiceServerNotFoundInAddService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("SERVICE_NAME: testservice\nSTATE: 4 RUNNING", nil)

	// сервер не найден при добавлении службы
	mockStorage.EXPECT().
		AddService(gomock.Any(), int64(1), int64(1), gomock.Any()).
		Return(nil, &errs.ErrServerNotFound{
			UserID:   1,
			ServerID: 1,
			Err:      errors.New("not found"),
		})

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Сервер не был найден")
}

// TestAddServiceDatabaseErrorInAddService Проверяет ошибку БД при AddService.
func TestAddServiceDatabaseErrorInAddService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("SERVICE_NAME: testservice\nSTATE: 4 RUNNING", nil)

	// обычная ошибка БД
	mockStorage.EXPECT().
		AddService(gomock.Any(), int64(1), int64(1), gomock.Any()).
		Return(nil, errors.New("database error"))

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка добавления службы")
}

// TestAddServiceSuccess Проверяет успешное добавление службы.
func TestAddServiceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	createdService := &models.Service{
		ID:            1,
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
		Status:        "running",
		UpdatedAt:     time.Now(),
	}

	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, time.Duration(0)).
		Return(true)

	mockClientFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), `sc query "testservice"`).
		Return("SERVICE_NAME: testservice\nSTATE: 4 RUNNING", nil)

	mockStorage.EXPECT().
		AddService(gomock.Any(), int64(1), int64(1), gomock.Any()).
		Return(createdService, nil)

	service := models.Service{
		ServiceName:   "testservice",
		DisplayedName: "Test Service",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем что ответ содержит службу
	var responseService models.Service
	err := json.NewDecoder(w.Body).Decode(&responseService)
	assert.NoError(t, err)
	assert.Equal(t, "testservice", responseService.ServiceName)
}

// TestAddServiceNameNormalization Проверяет нормализацию имени службы.
func TestAddServiceNameNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"с пробелами", "  MyService  "},
		{"с двойными кавычками", `"MyService"`},
		{"с одинарными кавычками", "'MyService'"},
		{"с обратными кавычками", "`MyService`"},
		{"с французскими кавычками", "«MyService»"},
		{"с английскими кавычками", "\"MyService\""},
		{"верхний регистр", "MYSERVICE"},
		{"смешанный регистр", "MySeRvIcE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
			mockChecker := netutilsMocks.NewMockChecker(ctrl)
			mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

			handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

			mockStorage.EXPECT().
				GetServerWithPassword(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, &errs.ErrServerNotFound{
					UserID:   1,
					ServerID: 1,
					Err:      errors.New("not found"),
				})

			service := models.Service{
				ServiceName:   tt.input,
				DisplayedName: "Test Service",
			}

			body, _ := json.Marshal(service)
			r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
			w := httptest.NewRecorder()

			ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
			ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
			ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
			r = r.WithContext(ctx)

			handler.AddService(w, r)

			// проверяем что обработка прошла
			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

// TestDelServiceSuccess Проверяет успешное удаление службы.
func TestDelServiceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		DelService(gomock.Any(), int64(1), int64(1), int64(1)).
		Return(nil)

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "успешно удалена")
}

// TestDelServiceNotFound Проверяет удаление несуществующей службы.
func TestDelServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		DelService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&errs.ErrServiceNotFound{
			UserID:    1,
			ServerID:  1,
			ServiceID: 1,
			Err:       errors.New("not found"),
		})

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "не найдена")
}

// TestDelServiceDatabaseError Проверяет ошибку БД при удалении.
func TestDelServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		DelService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServiceSuccess Проверяет успешное получение службы.
func TestGetServiceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	service := &models.Service{
		ID:            1,
		ServiceName:   "test-service",
		DisplayedName: "Test Service",
		Status:        "running",
	}

	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(1), int64(1), int64(1)).
		Return(service, nil)

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем ответ
	var responseService models.Service
	err := json.NewDecoder(w.Body).Decode(&responseService)
	assert.NoError(t, err)
	assert.Equal(t, "test-service", responseService.ServiceName)
}

// TestGetServiceNotFound Проверяет получение несуществующей службы.
func TestGetServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &errs.ErrServiceNotFound{
			UserID:    1,
			ServerID:  1,
			ServiceID: 1,
			Err:       errors.New("not found"),
		})

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "не найдена")
}

// TestGetServiceDatabaseError Проверяет ошибку БД.
func TestGetServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		GetService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database error"))

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))
	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServicesListSuccess Проверяет успешное получение списка.
func TestGetServicesListSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
		{ID: 2, ServiceName: "service2", DisplayedName: "Service 2", Status: "stopped"},
	}

	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем ответ
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseServices))
}

// TestGetServicesListEmpty Проверяет пустой список.
func TestGetServicesListEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return([]*models.Service{}, nil)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем пустой массив
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(responseServices))
}

// TestGetServicesListServerNotFound Проверяет ошибку "сервер не найден".
func TestGetServicesListServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		ListServices(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &errs.ErrServiceNotFound{
			ServerID: 1,
			Err:      errors.New("server not found"),
		})

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestGetServicesListDatabaseError Проверяет ошибку БД.
func TestGetServicesListDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	mockStorage.EXPECT().
		ListServices(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database error"))

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServicesListWithActualTrueServerNotFound Проверяет actual=true когда сервер не найден.
func TestGetServicesListWithActualTrueServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	// первый вызов - получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// второй вызов - получение сервера с паролем (ошибка)
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, &errs.ErrServerNotFound{
			UserID:   1,
			ServerID: 1,
			Err:      errors.New("server not found"),
		})

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Сервер не найден")
}

// TestGetServicesListWithActualTrueDatabaseError Проверяет actual=true при ошибке БД.
func TestGetServicesListWithActualTrueDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	// первый вызов - получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// второй вызов - ошибка БД
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(nil, errors.New("database error"))

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Ошибка при получении информации о сервере")
}

// TestGetServicesListWithActualTrueServerUnreachable Проверяет actual=true когда сервер недоступен.
func TestGetServicesListWithActualTrueServerUnreachable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	// получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// получение сервера
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	// сервер недоступен
	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, gomock.Any()).
		Return(false)

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "false", w.Header().Get("X-Is-Updated"))

	// проверяем что список служб вернулся
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(responseServices))
}

// TestGetServicesListWithActualTrueWorkerFailed Проверяет actual=true когда worker не смог обновить статусы.
func TestGetServicesListWithActualTrueWorkerFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	// получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// получение сервера
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	// сервер доступен
	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, gomock.Any()).
		Return(true)

	// worker не смог обновить статусы (возвращает false)
	mockStatusesWorker.EXPECT().
		CheckServicesStatuses(gomock.Any(), server, services).
		Return(nil, false)

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "false", w.Header().Get("X-Is-Updated"))

	// проверяем что список служб вернулся
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(responseServices))
}

// TestGetServicesListWithActualTrueBatchUpdateFailed Проверяет actual=true когда BatchChangeServiceStatus упал.
func TestGetServicesListWithActualTrueBatchUpdateFailed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	// обновлённые статусы служб
	updatedServices := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "stopped", UpdatedAt: time.Now()},
	}

	// получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// получение сервера
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	// сервер доступен
	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, gomock.Any()).
		Return(true)

	// worker успешно вернул обновлённые статусы
	mockStatusesWorker.EXPECT().
		CheckServicesStatuses(gomock.Any(), server, services).
		Return(updatedServices, true)

	// BatchChangeServiceStatus возвращает ошибку
	mockStorage.EXPECT().
		BatchChangeServiceStatus(gomock.Any(), int64(1), updatedServices).
		Return(errors.New("database error"))

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "false", w.Header().Get("X-Is-Updated"))

	// проверяем что список служб вернулся
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(responseServices))
}

// TestGetServicesListWithActualTrueSuccess Проверяет успешный сценарий actual=true.
func TestGetServicesListWithActualTrueSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	// обновлённые статусы служб
	updatedServices := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "stopped", UpdatedAt: time.Now()},
	}

	// получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// получение сервера
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), int64(1), int64(1)).
		Return(server, nil)

	// сервер доступен
	mockChecker.EXPECT().
		IsHostReachable("192.168.1.100", 5985, gomock.Any()).
		Return(true)

	// worker успешно вернул обновлённые статусы
	mockStatusesWorker.EXPECT().
		CheckServicesStatuses(gomock.Any(), server, services).
		Return(updatedServices, true)

	// BatchChangeServiceStatus успешен
	mockStorage.EXPECT().
		BatchChangeServiceStatus(gomock.Any(), int64(1), updatedServices).
		Return(nil)

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "true", w.Header().Get("X-Is-Updated"))

	// проверяем что список служб вернулся
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(responseServices))
}

// TestGetServicesListWithActualFalse Проверяет запрос без actual=true.
func TestGetServicesListWithActualFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
		{ID: 2, ServiceName: "service2", DisplayedName: "Service 2", Status: "stopped"},
	}

	// только один вызов - получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// GetServerWithPassword НЕ должен быть вызван
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// заголовок X-Is-Updated не должен быть установлен
	assert.Empty(t, w.Header().Get("X-Is-Updated"))

	// проверяем что список служб вернулся
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseServices))
}

// TestGetServicesListWithActualTrueEmptyServices Проверяет actual=true с пустым списком служб.
func TestGetServicesListWithActualTrueEmptyServices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	mockStatusesWorker := workerMocks.NewMockStatusesChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockClientFactory, mockChecker, mockStatusesWorker)

	// пустой список служб
	services := []*models.Service{}

	// только один вызов - получение списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// GetServerWithPassword НЕ должен быть вызван для пустого списка
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(0)

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем пустой массив
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(responseServices))
}

// TestIsServiceExistsVariants Проверяет все варианты isServiceExists.
func TestIsServiceExistsVariants(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		expected bool
	}{
		{
			name:     "с маркером STATE",
			result:   "SERVICE_NAME: test\nSTATE: 4 RUNNING",
			expected: true,
		},
		{
			name:     "с маркером SERVICE_NAME",
			result:   "SERVICE_NAME: someservice",
			expected: true,
		},
		{
			name:     "код ошибки 1060",
			result:   "QueryServiceConfig FAILED 1060",
			expected: false,
		},
		{
			name:     "пустой результат",
			result:   "",
			expected: false,
		},
		{
			name:     "неизвестный результат",
			result:   "Some random text",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isServiceExists(tt.result)
			assert.Equal(t, tt.expected, result)
		})
	}
}
