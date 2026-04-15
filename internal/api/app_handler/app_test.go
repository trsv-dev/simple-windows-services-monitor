package app_handler

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/auth/mocks"
	broadcasterMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewAppHandler Проверяет конструктор AppHandler.
func TestNewAppHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name              string
		setupAuthProvider func() *authMocks.MockAuthProvider
		setupBroadcaster  func() *broadcasterMocks.MockBroadcaster
		wantValidation    func(t *testing.T, handler *AppHandler)
	}{
		{
			name: "успешное создание обработчика с валидными зависимостями",
			setupAuthProvider: func() *authMocks.MockAuthProvider {
				return authMocks.NewMockAuthProvider(ctrl)
			},
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				assert.NotNil(t, handler.Broadcaster, "поле Broadcaster должно быть инициализировано")
				assert.NotNil(t, handler.AuthProvider, "поле AuthProvider должно быть инициализировано")
			},
		},
		{
			name: "создание обработчика с nil AuthProvider",
			setupAuthProvider: func() *authMocks.MockAuthProvider {
				return nil
			},
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				assert.Nil(t, handler.AuthProvider, "AuthProvider должен быть nil")
				assert.NotNil(t, handler.Broadcaster, "Broadcaster должен быть инициализирован")
			},
		},
		{
			name: "создание обработчика с nil Broadcaster",
			setupAuthProvider: func() *authMocks.MockAuthProvider {
				return authMocks.NewMockAuthProvider(ctrl)
			},
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return nil
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				assert.NotNil(t, handler.AuthProvider, "AuthProvider должен быть инициализирован")
				assert.Nil(t, handler.Broadcaster, "Broadcaster должен быть nil")
			},
		},
		{
			name: "проверка что все зависимости правильно передаются",
			setupAuthProvider: func() *authMocks.MockAuthProvider {
				return authMocks.NewMockAuthProvider(ctrl)
			},
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				require.NotNil(t, handler, "AppHandler должен быть создан")
				require.NotNil(t, handler.Broadcaster, "Broadcaster должно быть инициализировано")
				require.NotNil(t, handler.AuthProvider, "AuthProvider должно быть инициализировано")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка зависимостей
			mockAuthProvider := tt.setupAuthProvider()
			mockBroadcaster := tt.setupBroadcaster()

			// создаём AppHandler через конструктор
			handler := NewAppHandler(mockAuthProvider, mockBroadcaster)

			// проверяем результат создания
			tt.wantValidation(t, handler)
		})
	}
}

// TestAppHandlerFields Проверяет что все поля AppHandler инициализированы правильно.
func TestAppHandlerFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// подготовка
	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)
	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	// создаём обработчик
	handler := NewAppHandler(mockAuthProvider, mockBroadcaster)

	// проверяем все поля
	tests := []struct {
		name       string
		checkField func(t *testing.T)
	}{
		{
			name: "поле AuthProvider содержит моковый AuthProvider",
			checkField: func(t *testing.T) {
				assert.Equal(t, mockAuthProvider, handler.AuthProvider, "AuthProvider должен совпадать с переданным мок-объектом")
			},
		},
		{
			name: "поле Broadcaster содержит моковый broadcaster",
			checkField: func(t *testing.T) {
				assert.Equal(t, mockBroadcaster, handler.Broadcaster, "Broadcaster должен совпадать с переданным мок-объектом")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkField(t)
		})
	}
}

// TestNewAppHandlerDifferentDependencies Проверяет что разные зависимости корректно передаются.
func TestNewAppHandlerDifferentDependencies(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider1 := authMocks.NewMockAuthProvider(ctrl)
	mockAuthProvider2 := authMocks.NewMockAuthProvider(ctrl)
	broadcaster1 := broadcasterMocks.NewMockBroadcaster(ctrl)
	broadcaster2 := broadcasterMocks.NewMockBroadcaster(ctrl)

	handler1 := NewAppHandler(mockAuthProvider1, broadcaster1)
	handler2 := NewAppHandler(mockAuthProvider2, broadcaster2)

	// сохраняем адреса для проверки
	addrAuthProvider1 := fmt.Sprintf("%p", handler1.AuthProvider)
	addrAuthProvider2 := fmt.Sprintf("%p", handler2.AuthProvider)
	addrBroadcaster1 := fmt.Sprintf("%p", handler1.Broadcaster)
	addrBroadcaster2 := fmt.Sprintf("%p", handler2.Broadcaster)

	// проверяем что это разные объекты
	assert.NotEqual(t, addrAuthProvider1, addrAuthProvider2,
		"handler1.AuthProvider и handler2.AuthProvider должны быть разными объектами")
	assert.NotEqual(t, addrBroadcaster1, addrBroadcaster2,
		"handler1.Broadcaster и handler2.Broadcaster должны быть разными объектами")

	// проверяем что каждый handler имеет свои зависимости
	assert.NotNil(t, handler1.AuthProvider)
	assert.NotNil(t, handler2.AuthProvider)
	assert.NotNil(t, handler1.Broadcaster)
	assert.NotNil(t, handler2.Broadcaster)
}

// TestNewAppHandlerImmutability Проверяет что после создания поля не меняются.
func TestNewAppHandlerImmutability(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)
	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	handler := NewAppHandler(mockAuthProvider, mockBroadcaster)

	// сохраняем исходные значения
	originalAuthProvider := handler.AuthProvider
	originalBroadcaster := handler.Broadcaster

	// проверяем что значения не изменились после получения
	assert.Equal(t, mockAuthProvider, handler.AuthProvider)
	assert.Equal(t, mockBroadcaster, handler.Broadcaster)
	assert.Equal(t, originalAuthProvider, handler.AuthProvider)
	assert.Equal(t, originalBroadcaster, handler.Broadcaster)
}
