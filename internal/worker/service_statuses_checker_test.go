package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewServicesStatusesWorker Проверяет конструктор.
func TestNewServicesStatusesWorker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)

	worker := NewServiceStatusesChecker(mockFactory)

	assert.NotNil(t, worker)
}

// TestCheckServicesStatusesSuccess Проверяет успешное получение статусов служб.
func TestCheckServicesStatusesSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
		{ID: 2, ServiceName: "service2", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `[{"Name":"service1","Status":"Running"},{"Name":"service2","Status":"Stopped"}]`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	// реализация работает с мокированными зависимостями
	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 2, len(updates))
	assert.NotNil(t, updates[0].UpdatedAt)
	assert.NotNil(t, updates[1].UpdatedAt)
}

// TestCheckServicesStatusesClientFactoryError Проверяет ошибку при создании клиента.
func TestCheckServicesStatusesClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(nil, errors.New("connection failed"))

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.False(t, success)
	assert.Nil(t, updates)
}

// TestCheckServicesStatusesRunCommandError Проверяет ошибку при выполнении команды.
func TestCheckServicesStatusesRunCommandError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", errors.New("PowerShell error"))

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.False(t, success)
	assert.Nil(t, updates)
}

// TestCheckServicesStatusesEmptyArrayResponse Проверяет пустой массив от PowerShell.
func TestCheckServicesStatusesEmptyArrayResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("[]", nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 0, len(updates))
}

// TestCheckServicesStatusesSingleService Проверяет одну службу в ответе.
func TestCheckServicesStatusesSingleService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `{"Name":"service1","Status":"Running"}`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 1, len(updates))
}

// TestCheckServicesStatusesMixedCase Проверяет смешанный регистр имён служб.
func TestCheckServicesStatusesMixedCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "myservice", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `{"Name":"MyService","Status":"Running"}`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 1, len(updates))
	assert.Equal(t, "myservice", updates[0].ServiceName)
}

// TestCheckServicesStatusesUnmarshalError Проверяет ошибку при анмаршаллинге JSON.
func TestCheckServicesStatusesUnmarshalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `{invalid json}`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.False(t, success)
	assert.Nil(t, updates)
}

// TestCheckServicesStatusesEmptyResponse Проверяет пустой ответ от PowerShell.
func TestCheckServicesStatusesEmptyResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
	}

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return("", nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 0, len(updates))
}

// TestCheckServicesStatusesMultipleServicesPartialMatch Проверяет частичное совпадение служб.
func TestCheckServicesStatusesMultipleServicesPartialMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", Status: "unknown", UpdatedAt: time.Time{}},
		{ID: 2, ServiceName: "service2", Status: "unknown", UpdatedAt: time.Time{}},
		{ID: 3, ServiceName: "service3", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `[{"Name":"service1","Status":"Running"},{"Name":"service2","Status":"Stopped"}]`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 2, len(updates))
}

// TestCheckServicesStatusesServiceNameWithQuote Проверяет экранирование кавычек в имени службы.
func TestCheckServicesStatusesServiceNameWithQuote(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	server := &models.Server{
		ID:       1,
		Address:  "192.168.1.100",
		Username: "admin",
		Password: "password",
		Name:     "TestServer",
	}

	services := []*models.Service{
		{ID: 1, ServiceName: "service'name", Status: "unknown", UpdatedAt: time.Time{}},
	}

	psResponse := `{"Name":"service'name","Status":"Running"}`

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	mockClient.EXPECT().
		RunCommand(gomock.Any(), gomock.Any()).
		Return(psResponse, nil)

	worker := NewServiceStatusesChecker(mockFactory)
	ctx := context.Background()

	updates, success := worker.CheckServiceStatuses(ctx, server, services)

	assert.True(t, success)
	assert.Equal(t, 1, len(updates))
}
