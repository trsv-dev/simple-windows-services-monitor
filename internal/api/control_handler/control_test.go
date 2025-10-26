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

func createContextWithCreds(login string, userID, serverID, serviceID int64) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, contextkeys.Login, login)
	ctx = context.WithValue(ctx, contextkeys.ID, userID)
	ctx = context.WithValue(ctx, contextkeys.ServerID, serverID)
	ctx = context.WithValue(ctx, contextkeys.ServiceID, serviceID)
	return ctx
}

// TestServiceStop Проверяет остановку службы.
func TestServiceStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		login          string
		userID         int64
		serverID       int64
		serviceID      int64
		setupStorage   func(m *storageMocks.MockStorage)
		setupFactory   func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient)
		setupChecker   func(m *netutilsMock.MockChecker)
		wantStatus     int
		wantErrorResp  *response.APIError
		wantSuccessMsg string
	}{
		{
			name:      "сервер не найден",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(nil, errs.NewErrServerNotFound(100, 1, errors.New("not found")))
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {},
			setupChecker: func(m *netutilsMock.MockChecker) {},
			wantStatus:   http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Сервер не найден",
			},
		},
		{
			name:      "служба не найдена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(nil, errs.NewErrServiceNotFound(1, 100, 10, errors.New("not found")))
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {},
			setupChecker: func(m *netutilsMock.MockChecker) {},
			wantStatus:   http.StatusNotFound,
			wantErrorResp: &response.APIError{
				Code:    http.StatusNotFound,
				Message: "Служба не найдена",
			},
		},
		{
			name:      "сервер недоступен",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(false)
			},
			wantStatus: http.StatusBadGateway,
			wantErrorResp: &response.APIError{
				Code:    http.StatusBadGateway,
				Message: "Сервер недоступен",
			},
		},
		{
			name:      "ошибка создания WinRM клиента",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(nil, errors.New("auth failed"))
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка подключения к серверу",
			},
		},
		{
			name:      "успешная остановка запущенной службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Проверка статуса (запущена)
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 4 RUNNING", nil).
					Times(1)

				// Остановка
				client.EXPECT().
					RunCommand(gomock.Any(), `sc stop "TestService"`).
					Return("", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` остановлена",
		},
		{
			name:      "служба уже остановлена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Проверка статуса (уже остановлена)
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 1 STOPPED", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` уже остановлена",
		},
		{
			name:      "служба в процессе остановки",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Проверка статуса (в процессе остановки)
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 3 STOP_PENDING", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusConflict,
			wantErrorResp: &response.APIError{
				Code:    http.StatusConflict,
				Message: "Служба `Test Service` уже останавливается",
			},
		},
		{
			name:      "ошибка получения статуса службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Ошибка получения статуса
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("", errors.New("connection error")).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Не удалось получить статус службы `Test Service`",
			},
		},
		{
			name:      "ошибка при остановке службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: запущена
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 4 RUNNING", nil).
					Times(1)

				// Ошибка при остановке
				client.EXPECT().
					RunCommand(gomock.Any(), `sc stop "TestService"`).
					Return("", errors.New("permission denied")).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Не удалось остановить службу",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockChecker := netutilsMock.NewMockChecker(ctrl)
			mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
			mockClient := serviceControlMocks.NewMockClient(ctrl)

			tt.setupStorage(mockStorage)
			tt.setupFactory(mockClientFactory, mockClient)
			tt.setupChecker(mockChecker)

			handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker)

			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/stop", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServiceStop(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode, "статус должен совпадать")

			if tt.wantSuccessMsg != "" {
				var got response.APISuccess
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantSuccessMsg, got.Message)
			} else if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
				assert.Equal(t, tt.wantErrorResp.Message, got.Message)
			}
		})
	}
}

// TestServiceStart Проверяет запуск службы.
func TestServiceStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		login          string
		userID         int64
		serverID       int64
		serviceID      int64
		setupStorage   func(m *storageMocks.MockStorage)
		setupFactory   func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient)
		setupChecker   func(m *netutilsMock.MockChecker)
		wantStatus     int
		wantErrorResp  *response.APIError
		wantSuccessMsg string
	}{
		{
			name:      "успешный запуск остановленной службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: остановлена
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 1 STOPPED", nil).
					Times(1)

				// Запуск
				client.EXPECT().
					RunCommand(gomock.Any(), `sc start "TestService"`).
					Return("", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` запущена",
		},
		{
			name:      "служба уже запущена",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: уже запущена
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 4 RUNNING", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` уже запущена",
		},
		{
			name:      "служба в процессе запуска",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: в процессе запуска
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 2 START_PENDING", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusConflict,
			wantErrorResp: &response.APIError{
				Code:    http.StatusConflict,
				Message: "Служба `Test Service` уже запускается",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockChecker := netutilsMock.NewMockChecker(ctrl)
			mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
			mockClient := serviceControlMocks.NewMockClient(ctrl)

			tt.setupStorage(mockStorage)
			tt.setupFactory(mockClientFactory, mockClient)
			tt.setupChecker(mockChecker)

			handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker)

			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/start", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServiceStart(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantSuccessMsg != "" {
				var got response.APISuccess
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantSuccessMsg, got.Message)
			} else if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
				assert.Equal(t, tt.wantErrorResp.Message, got.Message)
			}
		})
	}
}

// TestServiceRestart Проверяет перезапуск службы.
func TestServiceRestart(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name           string
		login          string
		userID         int64
		serverID       int64
		serviceID      int64
		setupStorage   func(m *storageMocks.MockStorage)
		setupFactory   func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient)
		setupChecker   func(m *netutilsMock.MockChecker)
		wantStatus     int
		wantErrorResp  *response.APIError
		wantSuccessMsg string
	}{
		{
			name:      "успешный перезапуск запущенной службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				// Статус при остановке
				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Остановлена").
					Return(nil)

				// Статус при запуске
				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Первый запрос: статус (запущена)
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 4 RUNNING", nil).
					Times(1)

				// Остановка
				client.EXPECT().
					RunCommand(gomock.Any(), `sc stop "TestService"`).
					Return("", nil).
					Times(1)

				// Ожидание остановки (может быть несколько попыток)
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 1 STOPPED", nil).
					MinTimes(1).MaxTimes(5)

				// Запуск
				client.EXPECT().
					RunCommand(gomock.Any(), `sc start "TestService"`).
					Return("", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` перезапущена",
		},
		{
			name:      "перезапуск остановленной службы",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)

				mock.EXPECT().
					ChangeServiceStatus(gomock.Any(), int64(100), "TestService", "Работает").
					Return(nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: остановлена
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 1 STOPPED", nil).
					Times(1)

				// Запуск
				client.EXPECT().
					RunCommand(gomock.Any(), `sc start "TestService"`).
					Return("", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus:     http.StatusOK,
			wantSuccessMsg: "Служба `Test Service` перезапущена",
		},
		{
			name:      "служба в процессе изменения состояния",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: в процессе изменения
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 2 START_PENDING", nil).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusConflict,
			wantErrorResp: &response.APIError{
				Code:    http.StatusConflict,
				Message: "Служба `Test Service` уже изменяет состояние, попробуйте позже",
			},
		},
		{
			name:      "ошибка при остановке во время перезапуска",
			login:     "user",
			userID:    1,
			serverID:  100,
			serviceID: 10,
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetServerWithPassword(gomock.Any(), int64(100), int64(1)).
					Return(&models.Server{
						ID:       100,
						Name:     "TestServer",
						Address:  "192.168.1.1",
						Username: "admin",
						Password: "password",
					}, nil)

				mock.EXPECT().
					GetService(gomock.Any(), int64(100), int64(10), int64(1)).
					Return(&models.Service{
						ID:            10,
						ServiceName:   "TestService",
						DisplayedName: "Test Service",
					}, nil)
			},
			setupFactory: func(m *serviceControlMocks.MockClientFactory, client *serviceControlMocks.MockClient) {
				m.EXPECT().
					CreateClient("192.168.1.1", "admin", "password").
					Return(client, nil)

				// Статус: запущена
				client.EXPECT().
					RunCommand(gomock.Any(), `sc query "TestService"`).
					Return("STATE : 4 RUNNING", nil).
					Times(1)

				// Ошибка при остановке
				client.EXPECT().
					RunCommand(gomock.Any(), `sc stop "TestService"`).
					Return("", errors.New("permission denied")).
					Times(1)
			},
			setupChecker: func(m *netutilsMock.MockChecker) {
				m.EXPECT().
					IsHostReachable("192.168.1.1", 5985, time.Duration(0)).
					Return(true)
			},
			wantStatus: http.StatusInternalServerError,
			wantErrorResp: &response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Не удалось остановить службу `Test Service`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockChecker := netutilsMock.NewMockChecker(ctrl)
			mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)
			mockClient := serviceControlMocks.NewMockClient(ctrl)

			tt.setupStorage(mockStorage)
			tt.setupFactory(mockClientFactory, mockClient)
			tt.setupChecker(mockChecker)

			handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker)

			ctx := createContextWithCreds(tt.login, tt.userID, tt.serverID, tt.serviceID)
			r := httptest.NewRequest(http.MethodPost, "/service/restart", nil).WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServiceRestart(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantSuccessMsg != "" {
				var got response.APISuccess
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantSuccessMsg, got.Message)
			} else if tt.wantErrorResp != nil {
				var got response.APIError
				json.NewDecoder(res.Body).Decode(&got)
				assert.Equal(t, tt.wantErrorResp.Code, got.Code)
				assert.Equal(t, tt.wantErrorResp.Message, got.Message)
			}
		})
	}
}

// TestNewControlHandler Проверяет конструктор ControlHandler.
func TestNewControlHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockChecker := netutilsMock.NewMockChecker(ctrl)
	mockClientFactory := serviceControlMocks.NewMockClientFactory(ctrl)

	handler := NewControlHandler(mockStorage, mockClientFactory, mockChecker)

	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
	assert.NotNil(t, handler.clientFactory, "clientFactory должен быть инициализирован")
	assert.NotNil(t, handler.checker, "checker должен быть инициализирован")
}
