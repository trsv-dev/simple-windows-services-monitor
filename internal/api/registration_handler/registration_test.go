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

// errorReader Helper для эмуляции ошибки чтения тела запроса
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
	regKey := "reg-key"

	handler := NewRegistrationHandler(mockStorage, mockTokenBuilder, jwtSecret, regKey, false)

	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
	assert.NotNil(t, handler.tokenBuilder, "tokenBuilder должен быть инициализирован")
	assert.Equal(t, jwtSecret, handler.JWTSecretKey, "JWTSecretKey должен совпадать")
	assert.Equal(t, regKey, handler.registrationKey)
	assert.False(t, handler.openRegistration)
}

// TestUserRegistration Проверяет регистрацию пользователей.
func TestUserRegistration(t *testing.T) {
	testToken := "test-jwt-token"

	tests := []struct {
		name              string
		method            string
		body              io.Reader
		openRegistration  bool
		registrationKey   string
		setupStorage      func(m *storageMocks.MockStorage)
		setupTokenBuilder func(m *authMocks.MockTokenBuilder)
		wantStatus        int
		wantResponse      interface{}
		checkToken        bool
	}{
		{
			name:              "метод не POST",
			method:            http.MethodGet,
			body:              nil,
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusMethodNotAllowed,
			checkToken:        false,
		},
		{
			name:              "ошибка чтения тела запроса",
			method:            http.MethodPost,
			body:              &errorReader{},
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
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
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
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
				req := models.RegisterRequest{
					Login:           "",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - логин менее 4 символов",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "usr",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - пустой пароль",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "валидация - пароль менее 5 символов",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "pass",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "пользователь уже существует",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "existing",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
			name:   "закрытая регистрация - отсутствует регистрационный ключ",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "невалидный ключ регистрации",
			},
			checkToken: false,
		},
		{
			name:   "закрытая регистрация - неверный регистрационный ключ",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "wrong-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration:  false,
			registrationKey:   "valid-reg-key",
			setupStorage:      func(m *storageMocks.MockStorage) {},
			setupTokenBuilder: func(m *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "невалидный ключ регистрации",
			},
			checkToken: false,
		},
		{
			name:   "открытая регистрация - ключ не требуется",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: true,
			registrationKey:  "valid-reg-key",
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
		{
			name:   "успешная регистрация с верным ключом",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
		{
			name:   "логин с мин. допустимой длиной (4 символа)",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "user",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
				Login:   "user",
				Token:   testToken,
			},
			checkToken: true,
		},
		{
			name:   "пароль с мин. допустимой длиной (5 символов)",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "newuser",
					Password:        "pass1",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
		{
			name:   "логин со спецсимволами и цифрами",
			method: http.MethodPost,
			body: func() io.Reader {
				req := models.RegisterRequest{
					Login:           "user_123-test",
					Password:        "password123",
					RegistrationKey: "valid-reg-key",
				}
				b, _ := json.Marshal(req)
				return bytes.NewBuffer(b)
			}(),
			openRegistration: false,
			registrationKey:  "valid-reg-key",
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
				Login:   "user_123-test",
				Token:   testToken,
			},
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// новый gomock контроллер для каждого теста
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStorage := storageMocks.NewMockStorage(ctrl)
			mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)

			tt.setupStorage(mockStorage)
			tt.setupTokenBuilder(mockTokenBuilder)

			// используем параметры из теста
			handler := NewRegistrationHandler(
				mockStorage,
				mockTokenBuilder,
				"test-secret-key",
				tt.registrationKey,
				tt.openRegistration,
			)

			r := httptest.NewRequest(tt.method, "/register", tt.body)
			if tt.body != nil {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			handler.UserRegistration(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)

			if tt.wantResponse != nil {
				data, _ := io.ReadAll(res.Body)

				switch exp := tt.wantResponse.(type) {
				case response.APIError:
					var got response.APIError
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Code, got.Code)
					assert.Contains(t, got.Message, exp.Message)
				case response.AuthResponse:
					var got response.AuthResponse
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Message, got.Message)
					assert.Equal(t, exp.Login, got.Login)

					if tt.checkToken {
						assert.Equal(t, testToken, got.Token)
					}
				}
			}
		})
	}
}
