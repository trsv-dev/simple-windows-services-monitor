package authorization_handler

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

// TestNewAuthorizationHandler Проверяет конструктор AuthorizationHandler.
func TestNewAuthorizationHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	testJWTSecret := "test-secret-key"

	handler := NewAuthorizationHandler(mockStorage, mockTokenBuilder, testJWTSecret)

	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.NotNil(t, handler.storage, "storage должен быть инициализирован")
	assert.NotNil(t, handler.tokenBuilder, "tokenBuilder должен быть инициализирован")
	assert.Equal(t, testJWTSecret, handler.JWTSecretKey, "JWTSecretKey должен совпадать")
}

// errorReader - helper для эмуляции ошибки чтения тела запроса
type errorReader struct{}

func (er *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

// TestUserAuthorization Проверяет авторизацию пользователя.
func TestUserAuthorization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testToken := "test-jwt-token"

	tests := []struct {
		name              string                                 // название теста
		method            string                                 // HTTP метод
		body              io.Reader                              // тело запроса
		setupStorage      func(mock *storageMocks.MockStorage)   // настройка мока storage
		setupTokenBuilder func(mock *authMocks.MockTokenBuilder) // настройка мока TokenBuilder
		wantStatus        int                                    // ожидаемый HTTP статус
		wantResponse      interface{}                            // ожидаемый ответ
		checkToken        bool                                   // нужно ли проверять токен
	}{
		{
			name:              "метод не POST",
			method:            http.MethodGet,
			body:              nil,
			setupStorage:      func(mock *storageMocks.MockStorage) {},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusMethodNotAllowed,
			checkToken:        false,
		},
		{
			name:              "ошибка чтения тела запроса",
			method:            http.MethodPost,
			body:              &errorReader{},
			setupStorage:      func(mock *storageMocks.MockStorage) {},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Ошибка чтения тела запроса",
			},
			checkToken: false,
		},
		{
			name:              "невалидный JSON",
			method:            http.MethodPost,
			body:              bytes.NewBufferString("{invalid}"),
			setupStorage:      func(mock *storageMocks.MockStorage) {},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
			checkToken: false,
		},
		{
			name:   "валидация не пройдена - пустой логин",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage:      func(mock *storageMocks.MockStorage) {},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusBadRequest,
			checkToken:        false,
		},
		{
			name:   "неверные учётные данные",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil, errs.NewErrWrongLoginOrPassword(errors.New("bad")))
			},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusUnauthorized,
			wantResponse: response.APIError{
				Code:    http.StatusUnauthorized,
				Message: "Неверная пара логин/пароль",
			},
			checkToken: false,
		},
		{
			name:   "внутренняя ошибка хранилища",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewReader(b)
			}(),
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil, errors.New("db err"))
			},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {},
			wantStatus:        http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Внутренняя ошибка сервера",
			},
			checkToken: false,
		},
		{
			name:   "ошибка при создании JWT-токена",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewReader(b)
			}(),
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(&models.User{ID: 1, Login: "user"}, nil)
			},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {
				// TokenBuilder возвращает ошибку
				mock.EXPECT().
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
			name:   "успешная авторизация",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewReader(b)
			}(),
			setupStorage: func(mock *storageMocks.MockStorage) {
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(&models.User{ID: 1, Login: "user"}, nil)
			},
			setupTokenBuilder: func(mock *authMocks.MockTokenBuilder) {
				// TokenBuilder успешно создаёт токен
				mock.EXPECT().
					BuildJWTToken(gomock.AssignableToTypeOf(&models.User{}), "test-secret-key").
					Return(testToken, nil)
			},
			wantStatus: http.StatusOK,
			wantResponse: response.AuthResponse{
				Message: "Пользователь авторизован",
				Login:   "user",
				Token:   testToken,
			},
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготавливаем моки
			mockStore := storageMocks.NewMockStorage(ctrl)
			mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)

			tt.setupStorage(mockStore)
			tt.setupTokenBuilder(mockTokenBuilder)

			// создаём handler с моками
			handler := &AuthorizationHandler{
				storage:      mockStore,
				tokenBuilder: mockTokenBuilder,
				JWTSecretKey: "test-secret-key",
			}

			// создаём HTTP запрос
			r := httptest.NewRequest(tt.method, "/auth", tt.body)
			if tt.body != nil {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			// выполняем авторизацию
			handler.UserAuthorization(w, r)

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
					// проверяем успешный ответ авторизации
					var got response.AuthResponse
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Message, got.Message)
					assert.Equal(t, exp.Login, got.Login)

					// если нужно проверить токен - проверяем что он совпадает
					if tt.checkToken {
						assert.Equal(t, testToken, got.Token)
					}
				}
			}
		})
	}
}
