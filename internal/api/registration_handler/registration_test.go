package registration_handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	authMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/auth/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// errorReader - helper для эмуляции ошибки чтения тела запроса
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

// TestNewRegistrationHandler Проверяет конструктор RegistrationHandler.
func TestNewRegistrationHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	jwtSecret := "test-secret-key"

	handler := NewRegistrationHandler(mockStorage, mockTokenBuilder, jwtSecret)

	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
	assert.NotNil(t, handler.tokenBuilder, "tokenBuilder должен быть инициализирован")
	assert.Equal(t, jwtSecret, handler.JWTSecretKey, "JWTSecretKey должен совпадать")
}

// TestUserRegistration Проверяет регистрацию пользователей.
func TestUserRegistration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testToken := "test-jwt-token"

	tests := []struct {
		name              string                              // название теста
		method            string                              // HTTP метод
		body              io.Reader                           // тело запроса
		setupStorage      func(m *storageMocks.MockStorage)   // настройка мока storage
		setupTokenBuilder func(m *authMocks.MockTokenBuilder) // настройка мока TokenBuilder
		wantStatus        int                                 // ожидаемый HTTP статус
		wantResponse      interface{}                         // ожидаемый ответ
		checkToken        bool                                // нужно ли проверять токен
	}{
		{
			name:              "метод не POST",
			method:            http.MethodGet,
			body:              nil,
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusMethodNotAllowed,
			checkToken:        false,
		},
		{
			name:              "ошибка чтения тела запроса",
			method:            http.MethodPost,
			body:              &errorReader{},
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Ошибка чтения тела запроса",
			},
			checkToken: false,
		},
		{
			name:              "невалидный JSON",
			method:            http.MethodPost,
			body:              bytes.NewBufferString("{invalid}"),
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
			checkToken: false,
		},
		{
			name:   "валидация - пустой логин",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - логин менее 4 символов",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "usr", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - пустой пароль",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: ""}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - пароль менее 5 символов",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "pass"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "пользователь уже существует",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "existing", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(errs.NewErrLoginIsTaken("existing", errors.New("duplicate")))
			},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusConflict,
			wantResponse: response.APIError{
				Code:    http.StatusConflict,
				Message: "Пользователь уже существует",
			},
			checkToken: false,
		},
		{
			name:   "ошибка БД при создании пользователя",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "newuser", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(errors.New("db error"))
			},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusInternalServerError,
			checkToken:        false,
		},
		{
			name:   "ошибка при создании JWT-токена",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "newuser", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil)
			},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {
				m.EXPECT().
					BuildJWTToken(gomock.AssignableToTypeOf(&models.User{}), "test-secret-key").
					Return("", errors.New("signing failed"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка при создании JWT-токена",
			},
			checkToken: false,
		},
		{
			name:   "успешная регистрация",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "newuser", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil)
			},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {
				m.EXPECT().
					BuildJWTToken(gomock.AssignableToTypeOf(&models.User{}), "test-secret-key").
					Return(testToken, nil)
			},
			wantStatus: http.StatusCreated,
			wantResponse: response.AuthResponse{
				Message: "Пользователь зарегистрирован",
				Login:   "newuser",
				Token:   testToken,
			},
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготавливаем моки
			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)

			tt.setupStorage(mockStorage)
			tt.setupTokenBuilder(mockTokenBuilder)

			// создаём handler с моками
			handler := &RegistrationHandler{
				storage:      mockStorage,
				tokenBuilder: mockTokenBuilder,
				JWTSecretKey: "test-secret-key",
			}

			// создаём HTTP запрос
			r := httptest.NewRequest(tt.method, "/register", tt.body)
			if tt.body != nil {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			// выполняем регистрацию
			handler.UserRegistration(w, r)

			// проверка результатов
			res := w.Result()
			defer res.Body.Close()

			// проверяем HTTP статус код
			assert.Equal(t, tt.wantStatus, res.StatusCode)

			// проверяем тело ответа, если ожидается
			if tt.wantResponse != nil {
				data, _ := io.ReadAll(res.Body)

				switch exp := tt.wantResponse.(type) {
				case response.APIError:
					// проверяем ответ с ошибкой
					var got response.APIError
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Code, got.Code)
					assert.Contains(t, got.Message, exp.Message)
				case response.AuthResponse:
					// проверяем успешный ответ регистрации
					var got response.AuthResponse
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Message, got.Message)
					assert.Equal(t, exp.Login, got.Login)

					// если нужно проверить токен
					if tt.checkToken {
						assert.Equal(t, testToken, got.Token)
					}
				}
			}
		})
	}
}
