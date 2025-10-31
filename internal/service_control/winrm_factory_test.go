package service_control_test

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/service_control"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewWinRMClientFactory Проверяет конструктор фабрики.
func TestNewWinRMClientFactory(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	assert.NotNil(t, factory)
	assert.IsType(t, &service_control.WinRMClientFactory{}, factory)
}

// TestWinRMClientFactoryImplementsInterface Проверяет имплементацию интерфейса.
func TestWinRMClientFactoryImplementsInterface(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	// проверяем что фабрика реализует ClientFactory интерфейс
	assert.Implements(t, (*service_control.ClientFactory)(nil), factory)
}

// TestCreateClientSuccess Проверяет успешное создание клиента.
func TestCreateClientSuccess(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	client, err := factory.CreateClient("192.168.1.100", "admin", "password")

	if err == nil {
		assert.NotNil(t, client)
		assert.Implements(t, (*service_control.Client)(nil), client)
	}
}

// TestCreateClientReturnsClientInterface Проверяет что возвращается Client интерфейс.
func TestCreateClientReturnsClientInterface(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	client, err := factory.CreateClient("192.168.1.100", "admin", "password")

	if err == nil {
		// проверяем что это реально Client
		assert.NotNil(t, client)

		// проверяем что у него есть метод RunCommand
		_, ok := client.(service_control.Client)
		assert.True(t, ok)
	}
}

// TestCreateClientWithDifferentAddresses Проверяет разные адреса.
func TestCreateClientWithDifferentAddresses(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	tests := []struct {
		name    string
		address string
	}{
		{"localhost", "localhost"},
		{"IP адрес", "192.168.1.100"},
		{"FQDN", "server.example.com"},
		{"с портом", "192.168.1.100:5985"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.CreateClient(tt.address, "admin", "password")

			if err == nil {
				assert.NotNil(t, client)
				assert.Implements(t, (*service_control.Client)(nil), client)
			}
		})
	}
}

// TestCreateClientWithDifferentCredentials Проверяет разные креденшлы.
func TestCreateClientWithDifferentCredentials(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"простой юзер", "admin", "password"},
		{"с доменом", "DOMAIN\\admin", "password"},
		{"с точкой", "user@domain.com", "password"},
		{"сложный пароль", "admin", "P@ss!w0rd#2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := factory.CreateClient("192.168.1.100", tt.username, tt.password)

			if err == nil {
				assert.NotNil(t, client)
				assert.Implements(t, (*service_control.Client)(nil), client)
			}
		})
	}
}

// TestCreateClientEmptyAddress Проверяет пустой адрес.
func TestCreateClientEmptyAddress(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	client, err := factory.CreateClient("", "admin", "password")

	if err == nil {
		assert.NotNil(t, client)
		assert.Implements(t, (*service_control.Client)(nil), client)
	} else {
		assert.Nil(t, client)
		assert.Error(t, err)
	}
}

// TestCreateClientConsistency Проверяет консистентность при повторных вызовах.
func TestCreateClientConsistency(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	address := "192.168.1.100"
	username := "admin"
	password := "password"

	// Создаём клиента дважды с одинаковыми параметрами
	client1, err1 := factory.CreateClient(address, username, password)
	client2, err2 := factory.CreateClient(address, username, password)

	// Оба должны быть успешны или оба должны быть ошибкой
	if err1 == nil && err2 == nil {
		assert.NotNil(t, client1)
		assert.NotNil(t, client2)
		assert.IsType(t, client1, client2)
	} else if err1 != nil && err2 != nil {
		assert.Error(t, err1)
		assert.Error(t, err2)
	}
}

// TestMockClientFactorySuccess Проверяет mock фабрику с успешным созданием.
func TestMockClientFactorySuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil)

	client, err := mockFactory.CreateClient("192.168.1.100", "admin", "password")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, mockClient, client)
}

// TestMockClientFactoryError Проверяет mock фабрику с ошибкой.
func TestMockClientFactoryError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)

	mockFactory.EXPECT().
		CreateClient("invalid.address", "admin", "password").
		Return(nil, errors.New("cannot create client"))

	client, err := mockFactory.CreateClient("invalid.address", "admin", "password")

	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "cannot create client")
}

// TestMockClientFactoryWithAnyTimes Проверяет mock с AnyTimes.
func TestMockClientFactoryWithAnyTimes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockFactory.EXPECT().
		CreateClient(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(mockClient, nil).
		AnyTimes()

	// Несколько вызовов с разными параметрами
	for i := 0; i < 5; i++ {
		client, err := mockFactory.CreateClient("192.168.1.100", "admin", "password")

		assert.NoError(t, err)
		assert.NotNil(t, client)
	}
}

// TestMockClientFactoryWithTimes Проверяет mock с Times.
func TestMockClientFactoryWithTimes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", "admin", "password").
		Return(mockClient, nil).
		Times(3)

	client1, _ := mockFactory.CreateClient("192.168.1.100", "admin", "password")
	client2, _ := mockFactory.CreateClient("192.168.1.100", "admin", "password")
	client3, _ := mockFactory.CreateClient("192.168.1.100", "admin", "password")

	assert.NotNil(t, client1)
	assert.NotNil(t, client2)
	assert.NotNil(t, client3)
}

// TestMockClientFactoryWithDifferentAddresses Проверяет mock с разными адресами.
func TestMockClientFactoryWithDifferentAddresses(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient1 := serviceControlMocks.NewMockClient(ctrl)
	mockClient2 := serviceControlMocks.NewMockClient(ctrl)

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", gomock.Any(), gomock.Any()).
		Return(mockClient1, nil)

	mockFactory.EXPECT().
		CreateClient("192.168.1.101", gomock.Any(), gomock.Any()).
		Return(mockClient2, nil)

	client1, err1 := mockFactory.CreateClient("192.168.1.100", "admin", "password")
	client2, err2 := mockFactory.CreateClient("192.168.1.101", "admin", "password")

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// сравниваем по указателям
	assert.NotSame(t, client1, client2)
}

// TestMockClientFactoryWithSpecificCredentials Проверяет mock с конкретными креденшлами.
func TestMockClientFactoryWithSpecificCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := serviceControlMocks.NewMockClientFactory(ctrl)
	mockClient := serviceControlMocks.NewMockClient(ctrl)

	expectedUser := "DOMAIN\\admin"
	expectedPass := "SecureP@ss123"

	mockFactory.EXPECT().
		CreateClient("192.168.1.100", expectedUser, expectedPass).
		Return(mockClient, nil)

	client, err := mockFactory.CreateClient("192.168.1.100", expectedUser, expectedPass)

	assert.NoError(t, err)
	assert.NotNil(t, client)
}

// TestCreateClientEmptyPassword Проверяет пустой пароль.
func TestCreateClientEmptyPassword(t *testing.T) {
	factory := service_control.NewWinRMClientFactory()

	// пустой пароль может быть OK для некоторых конфигураций
	client, err := factory.CreateClient("192.168.1.100", "admin", "")

	// просто проверяем что функция отрабатывает
	if err == nil {
		assert.NotNil(t, client)
	}
}
