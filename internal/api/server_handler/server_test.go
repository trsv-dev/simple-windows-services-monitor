package server_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	serviceControlMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/service_control/mocks"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// Создание контекста с данными о пользователе и сервере.
func createContextWithCreds(login string, userID, serverID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextkeys.Login, login)
	ctx = context.WithValue(ctx, contextkeys.ID, userID)
	ctx = context.WithValue(ctx, contextkeys.ServerID, serverID)
	return ctx
}

// TestAddServer Проверяет добавление нового сервера.
func TestAddServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testFingerprint := uuid.New()

	tests := []struct {
		name               string
		login              string
		userID             int64
		body               interface{}
		setupFingerprinter func(m *serviceControlMocks.MockFingerprinter)
		setupStorage       func(m *storageMocks.MockStorage)
		wantStatus         int
		wantErrorResp      *response.APIError
		wantResponseFields []string
	}{
		{
			name:               "невалидный JSON",
			login:              "user",
			userID:             1,
			body:               "{invalid}",
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage:       func(m *storageMocks.MockStorage) {},
			wantStatus:         http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
		},
		{
			name:               "валидация не пройдена - пустой адрес",
			login:              "user",
			userID:             1,
			body:               models.Server{Name: "Test", Username: "admin", Password: "pass"},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage:       func(m *storageMocks.MockStorage) {},
			wantStatus:         http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "необходимо указать адрес сервера",
			},
		},
		{
			name:   "ошибка получения fingerprint",
			login:  "user",
			userID: 1,
			body: models.Server{
				Name:     "TestServer",
				Address:  "192.168.1.1",
				Username: "admin",
				Password: "password",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.1", "admin", "password").
					Return(uuid.Nil, errors.New("connection failed"))
			},
			setupStorage: func(m *storageMocks.MockStorage) {},
			wantStatus:   http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка получения UUID сервера",
			},
		},
		{
			name:   "дубликат сервера",
			login:  "user",
			userID: 1,
			body: models.Server{
				Name:     "TestServer",
				Address:  "192.168.1.1",
				Username: "admin",
				Password: "password",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.1", "admin", "password").
					Return(testFingerprint, nil)
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					AddServer(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil, errs.NewErrDuplicatedServer("192.168.1.1", errors.New("duplicate")))
			},
			wantStatus: http.StatusConflict,
			wantErrorResp: &response.APIError{
				Code:    http.StatusConflict,
				Message: "Сервер уже был добавлен",
			},
		},
		{
			name:   "ошибка БД при добавлении сервера",
			login:  "user",
			userID: 1,
			body: models.Server{
				Name:     "TestServer",
				Address:  "192.168.1.1",
				Username: "admin",
				Password: "password",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.1", "admin", "password").
					Return(testFingerprint, nil)
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					AddServer(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil, errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка добавления сервера",
			},
		},
		{
			name:   "успешное добавление сервера",
			login:  "user",
			userID: 1,
			body: models.Server{
				Name:     "TestServer",
				Address:  "192.168.1.1",
				Username: "admin",
				Password: "password",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.1", "admin", "password").
					Return(testFingerprint, nil)
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					AddServer(gomock.Any(), gomock.Any(), int64(1)).
					Return(&models.Server{
						ID:          1,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus:         http.StatusCreated,
			wantResponseFields: []string{"id", "name", "address", "username", "fingerprint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)
			mockStorage := storageMocks.NewMockStorage(ctrl)

			tt.setupFingerprinter(mockFingerprinter)
			tt.setupStorage(mockStorage)

			handler := NewServerHandler(mockStorage, mockFingerprinter)

			body, _ := json.Marshal(tt.body)
			r := httptest.NewRequest(http.MethodPost, "/servers", bytes.NewBuffer(body))
			r.Header.Set("Content-Type", "application/json")
			ctx := createContextWithCreds(tt.login, tt.userID, 0)
			r = r.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.AddServer(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
				assert.Equal(t, tt.wantErrorResp.Message, got.Message)
			} else if tt.wantResponseFields != nil {
				var got map[string]interface{}
				json.NewDecoder(res.Body).Decode(&got)
				for _, field := range tt.wantResponseFields {
					assert.NotNil(t, got[field], "поле %s должно присутствовать", field)
				}
			}
		})
	}
}

// TestEditServer Проверяет редактирование сервера.
func TestEditServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testFingerprint := uuid.New()

	tests := []struct {
		name               string
		login              string
		userID             int64
		serverID           int64
		body               interface{}
		setupFingerprinter func(m *serviceControlMocks.MockFingerprinter)
		setupStorage       func(m *storageMocks.MockStorage)
		wantStatus         int
		wantErrorResp      *response.APIError
	}{
		{
			name:               "невалидный JSON",
			login:              "user",
			userID:             1,
			serverID:           100,
			body:               "{invalid}",
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:      100,
						Name:    "TestServer",
						Address: "192.168.1.1",
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
		},
		{
			name:               "сервер не найден",
			login:              "user",
			userID:             1,
			serverID:           100,
			body:               models.Server{Name: "NewName"},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не был найден",
			},
		},
		{
			name:     "валидация - имя сервера слишком короткое",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Name: "ab", // менее 3 символов
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "необходимо указать имя сервера (минимум 3 символа)",
			},
		},
		{
			name:     "валидация - невалидный IP адрес",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address: "999.999.999.999", // невалидный IP
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "невалидный IP адрес: 999.999.999.999",
			},
		},
		{
			name:     "валидация - невалидный hostname",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address: "invalid hostname with spaces", // невалидный hostname
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "невалидный hostname: invalid hostname with spaces",
			},
		},
		{
			name:     "валидация - hostname слишком длинный",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address: strings.Repeat("a", 254), // 254+ символа
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "hostname слишком длинный:", // только часть сообщения
			},
		},
		{
			name:               "изменение только имени",
			login:              "user",
			userID:             1,
			serverID:           100,
			body:               models.Server{Name: "NewName"},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "NewName",
						Address:  "192.168.1.1",
						Username: "admin",
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "изменение адреса с несовпадающим fingerprint",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address:  "192.168.1.2",
				Username: "admin",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				differentFingerprint := uuid.New()
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.2", "admin", "password").
					Return(differentFingerprint, nil)
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Невозможно изменить адрес: UUID сервера `192.168.1.2` не совпадает с ранее зарегистрированным UUID `192.168.1.1`",
			},
		},
		{
			name:     "изменение пароля и адреса одновременно",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address:  "192.168.1.2",
				Username: "admin",
				Password: "newpassword",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				differentFingerprint := uuid.New()
				// должен быть вызван с новым паролем!
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.2", "admin", "newpassword").
					Return(differentFingerprint, nil)
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "oldpassword",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Невозможно изменить адрес: UUID сервера `192.168.1.2` не совпадает с ранее зарегистрированным UUID `192.168.1.1`",
			},
		},
		{
			name:     "изменение адреса с использованием старого пароля",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address:  "192.168.1.2",
				Username: "admin",
				// password не передаётся - должен использоваться старый
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				// должен быть вызван со старым паролем!
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.2", "admin", "oldpassword").
					Return(testFingerprint, nil) // совпадает с fingerprint старого адреса
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "oldpassword",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.2",
						Username: "admin",
						Password: "oldpassword",
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "изменение пароля без изменения адреса",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Password: "newpassword",
				// address не передаётся - GetFingerprint не должен вызваться
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				// getFingerprint вообще не должен быть вызван!
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "oldpassword",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "newpassword",
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "ошибка редактирования сервера - сервер не найден",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Name: "NewName",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:     "ошибка редактирования сервера - другая ошибка БД",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Name: "NewName",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(nil, errors.New("database connection error"))
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Ошибка при обновлении сервера",
			},
		},

		{
			name:     "ошибка при получении fingerprint с новым паролем",
			login:    "user",
			userID:   1,
			serverID: 100,
			body: models.Server{
				Address:  "192.168.1.2",
				Username: "admin",
				Password: "newpassword",
			},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {
				m.EXPECT().
					GetFingerprint(gomock.Any(), "192.168.1.2", "admin", "newpassword").
					Return(uuid.Nil, errors.New("invalid credentials"))
			},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "oldpassword",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка получения UUID сервера",
			},
		},
		{
			name:               "успешное редактирование",
			login:              "user",
			userID:             1,
			serverID:           100,
			body:               models.Server{Name: "NewName", Username: "newadmin"},
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Password:    "password",
						Fingerprint: testFingerprint,
					}, nil)

				m.EXPECT().
					EditServer(gomock.Any(), gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "NewName",
						Address:  "192.168.1.1",
						Username: "newadmin",
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)
			mockStorage := storageMocks.NewMockStorage(ctrl)

			tt.setupFingerprinter(mockFingerprinter)
			tt.setupStorage(mockStorage)

			handler := NewServerHandler(mockStorage, mockFingerprinter)

			body, _ := json.Marshal(tt.body)
			r := httptest.NewRequest(http.MethodPut, "/servers/100", bytes.NewBuffer(body))
			r.Header.Set("Content-Type", "application/json")
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID)
			r = r.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.EditServer(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
				assert.Contains(t, got.Message, tt.wantErrorResp.Message)
			}
		})
	}
}

// TestDelServer Проверяет удаление сервера.
func TestDelServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name               string
		login              string
		userID             int64
		serverID           int64
		setupFingerprinter func(m *serviceControlMocks.MockFingerprinter)
		setupStorage       func(m *storageMocks.MockStorage)
		wantStatus         int
		wantErrorResp      *response.APIError
	}{
		{
			name:               "сервер не найден",
			login:              "user",
			userID:             1,
			serverID:           100,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					DelServer(gomock.Any(), int64(100), int64(1)).
					Return(errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:               "ошибка БД при удалении",
			login:              "user",
			userID:             1,
			serverID:           100,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					DelServer(gomock.Any(), int64(100), int64(1)).
					Return(errors.New("db connection error"))
			},
			wantStatus: http.StatusBadRequest,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Ошибка при удалении сервера",
			},
		},
		{
			name:               "успешное удаление",
			login:              "user",
			userID:             1,
			serverID:           100,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					DelServer(gomock.Any(), int64(100), int64(1)).
					Return(nil)
			},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)
			mockStorage := storageMocks.NewMockStorage(ctrl)

			tt.setupFingerprinter(mockFingerprinter)
			tt.setupStorage(mockStorage)

			handler := NewServerHandler(mockStorage, mockFingerprinter)

			r := httptest.NewRequest(http.MethodDelete, "/servers/100", nil)
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID)
			r = r.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.DelServer(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
			}
		})
	}
}

// TestGetServer Проверяет получение информации о сервере.
func TestGetServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testFingerprint := uuid.New()

	tests := []struct {
		name               string
		login              string
		userID             int64
		serverID           int64
		setupFingerprinter func(m *serviceControlMocks.MockFingerprinter)
		setupStorage       func(m *storageMocks.MockStorage)
		wantStatus         int
		wantErrorResp      *response.APIError
	}{
		{
			name:               "сервер не найден",
			login:              "user",
			userID:             1,
			serverID:           100,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServer(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			wantStatus: http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:               "успешное получение",
			login:              "user",
			userID:             1,
			serverID:           100,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					GetServer(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:          100,
						Name:        "TestServer",
						Address:     "192.168.1.1",
						Username:    "admin",
						Fingerprint: testFingerprint,
					}, nil)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)
			mockStorage := storageMocks.NewMockStorage(ctrl)

			tt.setupFingerprinter(mockFingerprinter)
			tt.setupStorage(mockStorage)

			handler := NewServerHandler(mockStorage, mockFingerprinter)

			r := httptest.NewRequest(http.MethodGet, "/servers/100", nil)
			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID)
			r = r.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.GetServer(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
			}
		})
	}
}

// TestGetServerList Проверяет получение списка серверов.
func TestGetServerList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testFingerprint := uuid.New()

	tests := []struct {
		name               string
		login              string
		userID             int64
		setupFingerprinter func(m *serviceControlMocks.MockFingerprinter)
		setupStorage       func(m *storageMocks.MockStorage)
		wantStatus         int
		wantServerCount    int
	}{
		{
			name:               "ошибка при получении списка",
			login:              "user",
			userID:             1,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return(nil, errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:               "пустой список серверов",
			login:              "user",
			userID:             1,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return([]*models.Server{}, nil)
			},
			wantStatus:      http.StatusOK,
			wantServerCount: 0,
		},
		{
			name:               "список с серверами",
			login:              "user",
			userID:             1,
			setupFingerprinter: func(m *serviceControlMocks.MockFingerprinter) {},
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					ListServers(gomock.Any(), int64(1)).
					Return([]*models.Server{
						{
							ID:          1,
							Name:        "Server1",
							Address:     "192.168.1.1",
							Fingerprint: testFingerprint,
						},
						{
							ID:          2,
							Name:        "Server2",
							Address:     "192.168.1.2",
							Fingerprint: testFingerprint,
						},
					}, nil)
			},
			wantStatus:      http.StatusOK,
			wantServerCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)
			mockStorage := storageMocks.NewMockStorage(ctrl)

			tt.setupFingerprinter(mockFingerprinter)
			tt.setupStorage(mockStorage)

			handler := NewServerHandler(mockStorage, mockFingerprinter)

			r := httptest.NewRequest(http.MethodGet, "/servers", nil)
			ctx := context.Background()
			ctx = context.WithValue(ctx, contextkeys.ID, tt.userID)
			r = r.WithContext(ctx)

			w := httptest.NewRecorder()
			handler.GetServerList(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantStatus == http.StatusOK {
				var got []*models.Server
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantServerCount, len(got))
			}
		})
	}
}

// TestNewServerHandler Проверяет конструктор ServerHandler.
func TestNewServerHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockFingerprinter := serviceControlMocks.NewMockFingerprinter(ctrl)

	handler := NewServerHandler(mockStorage, mockFingerprinter)

	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
	assert.NotNil(t, handler.fingerprinter, "fingerprinter должен быть инициализирован")
}
