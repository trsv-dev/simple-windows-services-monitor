package api

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	broadcasterMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
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
		name           string                                                                // название теста
		setupMocks     func() (*storageMocks.MockStorage, *broadcasterMocks.MockBroadcaster) // настройка зависимостей
		jwtSecretKey   string                                                                // JWT секретный ключ
		wantValidation func(t *testing.T, handler *AppHandler)                               // функция проверки результата
	}{
		{
			name: "успешное создание обработчика с валидными параметрами",
			setupMocks: func() (*storageMocks.MockStorage, *broadcasterMocks.MockBroadcaster) {
				// создаём моки зависимостей
				mockStorage := storageMocks.NewMockStorage(ctrl)
				mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
				return mockStorage, mockBroadcaster
			},
			jwtSecretKey: "test-secret-key-123",
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что обработчик не nil
				assert.NotNil(t, handler, "AppHandler не должен быть nil")
				// проверяем что storage инициализирован
				assert.NotNil(t, handler.storage, "поле storage должно быть инициализировано")
				// проверяем что broadcaster инициализирован
				assert.NotNil(t, handler.Broadcaster, "поле Broadcaster должно быть инициализировано")
				// проверяем что JWT секретный ключ установлен правильно
				assert.Equal(t, "test-secret-key-123", handler.JWTSecretKey, "JWT ключ должен совпадать")
			},
		},
		{
			name: "создание обработчика с пустым JWT ключом",
			setupMocks: func() (*storageMocks.MockStorage, *broadcasterMocks.MockBroadcaster) {
				// создаём моки зависимостей
				mockStorage := storageMocks.NewMockStorage(ctrl)
				mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
				return mockStorage, mockBroadcaster
			},
			jwtSecretKey: "",
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что обработчик создан корректно
				assert.NotNil(t, handler, "AppHandler не должен быть nil даже с пустым ключом")
				// проверяем что storage инициализирован
				assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
				// проверяем что broadcaster инициализирован
				assert.NotNil(t, handler.Broadcaster, "Broadcaster должен быть инициализирован")
				// проверяем что JWT ключ пустой
				assert.Empty(t, handler.JWTSecretKey, "JWT ключ должен быть пустым")
			},
		},
		{
			name: "создание обработчика с длинным сложным JWT ключом",
			setupMocks: func() (*storageMocks.MockStorage, *broadcasterMocks.MockBroadcaster) {
				// создаём моки зависимостей
				mockStorage := storageMocks.NewMockStorage(ctrl)
				mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
				return mockStorage, mockBroadcaster
			},
			jwtSecretKey: "very-long-secret-key-with-many-symbols-1234567890-!@#$%^&*()",
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
			name: "проверка что все зависимости правильно передаются",
			setupMocks: func() (*storageMocks.MockStorage, *broadcasterMocks.MockBroadcaster) {
				// создаём моки зависимостей
				mockStorage := storageMocks.NewMockStorage(ctrl)
				mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
				return mockStorage, mockBroadcaster
			},
			jwtSecretKey: "secret",
			wantValidation: func(t *testing.T, handler *AppHandler) {
				// проверяем что все компоненты инициализированы
				require.NotNil(t, handler, "AppHandler должен быть создан")
				require.NotNil(t, handler.storage, "storage должно быть инициализировано")
				require.NotNil(t, handler.Broadcaster, "Broadcaster должно быть инициализировано")
				require.NotEmpty(t, handler.JWTSecretKey, "JWTSecretKey должен быть установлен")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка зависимостей
			mockStorage, mockBroadcaster := tt.setupMocks()

			// создаём AppHandler через конструктор
			handler := NewAppHandler(mockStorage, tt.jwtSecretKey, mockBroadcaster)

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
	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)
	testJWTKey := "test-key"

	// создаём обработчик
	handler := NewAppHandler(mockStorage, testJWTKey, mockBroadcaster)

	// проверяем все поля
	tests := []struct {
		name       string             // название проверки
		checkField func(t *testing.T) // функция проверки поля
	}{
		{
			name: "поле storage содержит моковое хранилище",
			checkField: func(t *testing.T) {
				assert.Equal(t, mockStorage, handler.storage, "storage должно совпадать с переданным мок-объектом")
			},
		},
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
