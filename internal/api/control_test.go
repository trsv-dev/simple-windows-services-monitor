package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	broadcasterMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	// инициализируем логгер для избежания nil pointer dereference в тестах
	logger.InitLogger("error", "stdout")
}

// createContextWithCreds Вспомогательная функция для создания контекста с учётными данными.
func createContextWithCreds(login string, userID, serverID, serviceID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextkeys.Login, login)
	ctx = context.WithValue(ctx, contextkeys.ID, userID)
	ctx = context.WithValue(ctx, contextkeys.ServerID, serverID)
	ctx = context.WithValue(ctx, contextkeys.ServiceID, serviceID)
	return ctx
}

// TestServiceStop Проверяет остановку службы.
// Тестирует только часть функционала (до начала работы с WinRM клиентом).
func TestServiceStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string                            // название теста
		login        string                            // логин пользователя
		userID       int64                             // ID пользователя
		serverID     int64                             // ID сервера
		serviceID    int64                             // ID службы
		setupMock    func(m *storageMocks.MockStorage) // настройка мока storage
		wantStatus   int                               // ожидаемый HTTP статус
		wantResponse response.APIError                 // ожидаемый ответ с ошибкой
	}{
		{
			name:      "сервер не найден",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с ошибкой "сервер не найден"
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:      "внутренняя ошибка при получении сервера",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с общей ошибкой
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о сервере",
			},
		},
		{
			name:      "служба не найдена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с ошибкой "служба не найдена"
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errs.NewErrServiceNotFound(1, 100, 10, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Служба не найдена",
			},
		},
		{
			name:      "внутренняя ошибка при получении службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с общей ошибкой
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о службе",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)
			mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

			// создаём handler с моками
			handler := &AppHandler{
				storage:      mockStorage,
				JWTSecretKey: "test-secret-key",
				Broadcaster:  mockBroadcaster,
			}

			// создаём контекст с учётными данными используя contextkeys
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			// выполнение остановки службы
			handler.ServiceStop(w, r)

			// проверка результатов
			res := w.Result()
			defer res.Body.Close()

			// проверяем HTTP статус код
			assert.Equal(t, tt.wantStatus, res.StatusCode, "HTTP статус должен совпадать")

			// проверяем тело ответа с ошибкой
			var got response.APIError
			json.NewDecoder(res.Body).Decode(&got)
			assert.Equal(t, tt.wantResponse.Code, got.Code, "код ошибки должен совпадать")
			assert.Equal(t, tt.wantResponse.Message, got.Message, "сообщение ошибки должно совпадать")
		})
	}
}

// TestServiceStart Проверяет запуск службы.
// Тестирует только часть функционала (до начала работы с WinRM клиентом).
func TestServiceStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string                            // название теста
		login        string                            // логин пользователя
		userID       int64                             // ID пользователя
		serverID     int64                             // ID сервера
		serviceID    int64                             // ID службы
		setupMock    func(m *storageMocks.MockStorage) // настройка мока storage
		wantStatus   int                               // ожидаемый HTTP статус
		wantResponse response.APIError                 // ожидаемый ответ с ошибкой
	}{
		{
			name:      "сервер не найден",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с ошибкой "сервер не найден"
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:      "внутренняя ошибка при получении сервера",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с общей ошибкой
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о сервере",
			},
		},
		{
			name:      "служба не найдена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с ошибкой "служба не найдена"
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errs.NewErrServiceNotFound(1, 100, 10, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Служба не найдена",
			},
		},
		{
			name:      "внутренняя ошибка при получении службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с общей ошибкой
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о службе",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)
			mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

			// создаём handler с моками
			handler := &AppHandler{
				storage:      mockStorage,
				JWTSecretKey: "test-secret-key",
				Broadcaster:  mockBroadcaster,
			}

			// создаём контекст с учётными данными используя contextkeys
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			// выполнение запуска службы
			handler.ServiceStart(w, r)

			// проверка результатов
			res := w.Result()
			defer res.Body.Close()

			// проверяем HTTP статус код
			assert.Equal(t, tt.wantStatus, res.StatusCode, "HTTP статус должен совпадать")

			// проверяем тело ответа с ошибкой
			var got response.APIError
			json.NewDecoder(res.Body).Decode(&got)
			assert.Equal(t, tt.wantResponse.Code, got.Code, "код ошибки должен совпадать")
			assert.Equal(t, tt.wantResponse.Message, got.Message, "сообщение ошибки должно совпадать")
		})
	}
}

// TestServiceRestart Проверяет перезапуск службы.
// Тестирует только часть функционала (до начала работы с WinRM клиентом).
func TestServiceRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string                            // название теста
		login        string                            // логин пользователя
		userID       int64                             // ID пользователя
		serverID     int64                             // ID сервера
		serviceID    int64                             // ID службы
		setupMock    func(m *storageMocks.MockStorage) // настройка мока storage
		wantStatus   int                               // ожидаемый HTTP статус
		wantResponse response.APIError                 // ожидаемый ответ с ошибкой
	}{
		{
			name:      "сервер не найден",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с ошибкой "сервер не найден"
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:      "внутренняя ошибка при получении сервера",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов GetServerWithPassword с общей ошибкой
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о сервере",
			},
		},
		{
			name:      "служба не найдена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с ошибкой "служба не найдена"
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errs.NewErrServiceNotFound(1, 100, 10, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantResponse: response.APIError{
				Code:    http.StatusNotFound,
				Message: "Служба не найдена",
			},
		},
		{
			name:      "внутренняя ошибка при получении службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupMock: func(mock *storageMocks.MockStorage) {
				// успешно возвращаем сервер
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				// ожидаем вызов GetService с общей ошибкой
				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при получении информации о службе",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)
			mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

			// создаём handler с моками
			handler := &AppHandler{
				storage:      mockStorage,
				JWTSecretKey: "test-secret-key",
				Broadcaster:  mockBroadcaster,
			}

			// создаём контекст с учётными данными используя contextkeys
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			// выполнение перезапуска службы
			handler.ServiceRestart(w, r)

			// проверка результатов
			res := w.Result()
			defer res.Body.Close()

			// проверяем HTTP статус код
			assert.Equal(t, tt.wantStatus, res.StatusCode, "HTTP статус должен совпадать")

			// проверяем тело ответа с ошибкой
			var got response.APIError
			json.NewDecoder(res.Body).Decode(&got)
			assert.Equal(t, tt.wantResponse.Code, got.Code, "код ошибки должен совпадать")
			assert.Equal(t, tt.wantResponse.Message, got.Message, "сообщение ошибки должно совпадать")
		})
	}
}
