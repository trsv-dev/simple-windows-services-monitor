package app_handler

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	broadcasterMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	// инициализируем логгер для избежания nil pointer dereference в тестах
	logger.InitLogger("error", "stdout")
}

// TestNewAppHandler Проверяет конструктор AppHandler.
func TestNewAppHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name             string
		jwtSecretKey     string
		setupBroadcaster func() *broadcasterMocks.MockBroadcaster
		wantValidation   func(t *testing.T, handler *AppHandler)
	}{
		{
			name:         "успешное создание обработчика с валидными параметрами",
			jwtSecretKey: "test-secret-key-123",
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что обработчик не nil
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				// проверяем что broadcaster инициализирован
				assert.NotNil(t, handler.Broadcaster, "поле Broadcaster должно быть инициализировано")
				// проверяем что JWT секретный ключ установлен правильно
				assert.Equal(t, "test-secret-key-123", handler.JWTSecretKey, "JWT ключ должен совпадать")
			},
		},
		{
			name:         "создание обработчика с пустым JWT ключом",
			jwtSecretKey: "",
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что обработчик создан корректно
				assert.NotNil(t, handler, "AppHandler не должен быть nil даже с пустым ключом")
				// проверяем что broadcaster инициализирован
				assert.NotNil(t, handler.Broadcaster, "Broadcaster должен быть инициализирован")
				// проверяем что JWT ключ пустой
				assert.Empty(t, handler.JWTSecretKey, "JWT ключ должен быть пустым")
			},
		},
		{
			name:         "создание обработчика с длинным сложным JWT ключом",
			jwtSecretKey: "very-long-secret-key-with-many-symbols-1234567890-!@#$%^&*()",
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что обработчик создан
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				// проверяем что длинный JWT ключ установлен правильно
				assert.Equal(
					t,
					"very-long-secret-key-with-many-symbols-1234567890-!@#$%^&*()",
					handler.JWTSecretKey,
					"длинный JWT ключ должен сохраниться полностью",
				)
			},
		},
		{
			name:         "проверка что все зависимости правильно передаются",
			jwtSecretKey: "secret",
			setupBroadcaster: func() *broadcasterMocks.MockBroadcaster {
				return broadcasterMocks.NewMockBroadcaster(ctrl)
			},
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что все компоненты инициализированы
				require.NotNil(t, handler, "AppHandler должен быть создан")
				require.NotNil(t, handler.Broadcaster, "Broadcaster должно быть инициализировано")
				require.NotEmpty(t, handler.JWTSecretKey, "JWTSecretKey должен быть установлен")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка зависимостей
			mockBroadcaster := tt.setupBroadcaster()

			// создаём AppHandler через конструктор
			handler := NewAppHandler(tt.jwtSecretKey, mockBroadcaster)

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
	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
	testJWTKey := "test-key"

	// создаём обработчик
	handler := NewAppHandler(testJWTKey, mockBroadcaster)

	// проверяем все поля
	tests := []struct {
		name       string
		checkField func(t *testing.T)
	}{
		{
			name: "поле JWTSecretKey содержит правильное значение",
			checkField: func(t *testing.T) {
				assert.Equal(t, testJWTKey, handler.JWTSecretKey, "JWTSecretKey должен совпадать")
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

// TestNewAppHandlerDifferentBroadcasters Проверяет что разные broadcaster-ы корректно передаются.
func TestNewAppHandlerDifferentBroadcasters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	broadcaster1 := broadcasterMocks.NewMockBroadcaster(ctrl)
	broadcaster2 := broadcasterMocks.NewMockBroadcaster(ctrl)
	jwtKey := "secret"

	handler1 := NewAppHandler(jwtKey, broadcaster1)
	handler2 := NewAppHandler(jwtKey, broadcaster2)

	// сохраняем адреса для проверки
	addr1 := fmt.Sprintf("%p", handler1.Broadcaster)
	addr2 := fmt.Sprintf("%p", handler2.Broadcaster)

	// проверяем что это разные объекты
	assert.NotEqual(t, addr1, addr2,
		"handler1.Broadcaster и handler2.Broadcaster должны быть разными объектами")

	// проверяем что каждый handler имеет свой broadcaster
	assert.NotNil(t, handler1.Broadcaster)
	assert.NotNil(t, handler2.Broadcaster)
}

// TestNewAppHandlerImmutability Проверяет что после создания поля не меняются.
func TestNewAppHandlerImmutability(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
	testJWTKey := "original-key"

	handler := NewAppHandler(testJWTKey, mockBroadcaster)

	// сохраняем исходные значения
	originalJWTKey := handler.JWTSecretKey
	originalBroadcaster := handler.Broadcaster

	// проверяем что значения не изменились после получения
	assert.Equal(t, testJWTKey, handler.JWTSecretKey)
	assert.Equal(t, mockBroadcaster, handler.Broadcaster)
	assert.Equal(t, originalJWTKey, handler.JWTSecretKey)
	assert.Equal(t, originalBroadcaster, handler.Broadcaster)
}
