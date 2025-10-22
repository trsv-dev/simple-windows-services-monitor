package api

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

// TestUserRegistration Проверяет регистрацию пользователей.
func TestUserRegistration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string                            // название теста
		method       string                            // HTTP метод
		body         io.Reader                         // тело запроса
		setupMock    func(m *storageMocks.MockStorage) // настройка мока storage
		wantStatus   int                               // ожидаемый HTTP статус
		wantResponse interface{}                       // ожидаемый ответ
		checkToken   bool                              // нужно ли проверять наличие токена
	}{
		{
			name:       "метод не POST",
			method:     http.MethodGet,
			body:       nil,
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusMethodNotAllowed,
			checkToken: false,
		},
		{
			name:       "невалидный JSON",
			method:     http.MethodPost,
			body:       bytes.NewBufferString("{invalid}"),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
			checkToken: false,
		},
		{
			name:       "ошибка чтения тела запроса",
			method:     http.MethodPost,
			body:       &errorReader{},
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Ошибка чтения тела запроса",
			},
			checkToken: false,
		},
		{
			name:   "пустой логин при валидации",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "передан слишком короткий логин (менее 4 символов)",
			},
			checkToken: false,
		},
		{
			name:   "логин менее 4 символов при валидации",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "передан слишком короткий логин (менее 4 символов)",
			},
			checkToken: false,
		},
		{
			name:   "пустой пароль при валидации",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: ""}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			checkToken: false,
		},
		{
			name:   "пароль менее 5 символов при валидации",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: ""}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			checkToken: false,
		},
		{
			name:   "пользователь уже существует",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "existing_user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов CreateUser с возвратом ошибки дубликата логина
				mock.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(errs.NewErrLoginIsTaken("existing_user", errors.New("duplicate key")))
			},
			wantStatus: http.StatusConflict,
			wantResponse: response.APIError{
				Code:    http.StatusConflict,
				Message: "Пользователь уже существует",
			},
			checkToken: false,
		},
		{
			name:   "внутренняя ошибка хранилища",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "new_user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем вызов CreateUser с возвратом неизвестной ошибки
				mock.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(errors.New("database error"))
			},
			wantStatus: http.StatusInternalServerError,
			checkToken: false,
		},
		{
			name:   "успешная регистрация",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "new_user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock: func(mock *storageMocks.MockStorage) {
				// ожидаем успешный CreateUser
				mock.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil)
			},
			wantStatus: http.StatusCreated,
			wantResponse: response.AuthResponse{
				Message: "Пользователь зарегистрирован",
				Login:   "new_user",
				Token:   "", // будет проверяться отдельно через checkToken
			},
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange - подготовка
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)
			mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

			// создаём handler с моками
			handler := &AppHandler{
				storage:      mockStorage,
				JWTSecretKey: "test-secret-key",
				Broadcaster:  mockBroadcaster,
			}

			// создаём HTTP запрос
			r := httptest.NewRequest(tt.method, "/register", tt.body)
			if tt.body != nil {
				r.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()

			// выполнение регистрации
			handler.UserRegistration(w, r)

			// проверка результатов
			res := w.Result()
			defer res.Body.Close()

			// проверяем HTTP статус-код
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
					assert.Equal(t, exp.Message, got.Message)
				case response.AuthResponse:
					// проверяем успешный ответ регистрации
					var got response.AuthResponse
					json.Unmarshal(data, &got)
					assert.Equal(t, exp.Message, got.Message)
					assert.Equal(t, exp.Login, got.Login)

					// если нужно проверить токен - проверяем что он не пустой
					if tt.checkToken {
						assert.NotEmpty(t, got.Token, "токен не должен быть пустым")
					}
				}
			}
		})
	}
}

// TestUserRegistrationHeaders Проверяет установку заголовков при успешной регистрации.
func TestUserRegistrationHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// подготовка
	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockStorage.EXPECT().
		CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
		Return(nil)

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	handler := &AppHandler{
		storage:      mockStorage,
		JWTSecretKey: "test-secret-key",
		Broadcaster:  mockBroadcaster,
	}

	u := models.User{Login: "user", Password: "password"}
	b, _ := json.Marshal(u)
	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// выполнение регистрации
	handler.UserRegistration(w, r)

	// проверка заголовков
	res := w.Result()
	defer res.Body.Close()

	tests := []struct {
		name        string // название проверки
		checkHeader func(t *testing.T)
	}{
		{
			name: "заголовок Content-Type установлен на application/json",
			checkHeader: func(t *testing.T) {
				assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
			},
		},
		{
			name: "заголовок Set-Cookie содержит JWT токен",
			checkHeader: func(t *testing.T) {
				cookie := res.Header.Get("Set-Cookie")
				assert.NotEmpty(t, cookie, "cookie должна быть установлена")
				assert.Contains(t, cookie, "JWT=", "cookie должна содержать JWT=")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkHeader(t)
		})
	}
}

// errorReader - вспомогательная структура для имитации ошибки при чтении
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
