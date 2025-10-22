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
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	// инициализируем логгер для избежания nil pointer dereference в тестах
	logger.InitLogger("error", "stdout")
}

// TestUserAuthorization Проверяет авторизацию пользователя.
func TestUserAuthorization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name         string                        // название теста
		method       string                        // HTTP метод
		body         io.Reader                     // тело запроса
		setupMock    func(mock *mocks.MockStorage) // настройка мока storage
		wantStatus   int                           // ожидаемый HTTP статус
		wantResponse interface{}                   // ожидаемый ответ
		checkToken   bool                          // нужно ли проверять токен
	}{
		{
			name:       "метод не POST",
			method:     http.MethodGet,
			body:       nil,
			setupMock:  func(mock *mocks.MockStorage) {},
			wantStatus: http.StatusMethodNotAllowed,
			checkToken: false,
		},
		{
			name:       "невалидный JSON",
			method:     http.MethodPost,
			body:       bytes.NewBufferString("{invalid}"),
			setupMock:  func(mock *mocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
			wantResponse: response.APIError{
				Code:    http.StatusBadRequest,
				Message: "Неверный формат запроса",
			},
			checkToken: false,
		},
		{
			name:   "неверные учётные данные",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "user", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock: func(mock *mocks.MockStorage) {
				// ожидаем вызов GetUser с возвратом ошибки неверных учётных данных
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil, errs.NewErrWrongLoginOrPassword(errors.New("bad")))
			},
			wantStatus: http.StatusUnauthorized,
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
			setupMock: func(mock *mocks.MockStorage) {
				// ожидаем вызов GetUser с возвратом ошибки БД
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil, errors.New("db err"))
			},
			wantStatus: http.StatusInternalServerError,
			wantResponse: response.APIError{
				Code:    http.StatusInternalServerError,
				Message: "Внутренняя ошибка сервера",
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
			setupMock: func(mock *mocks.MockStorage) {
				// ожидаем успешный GetUser
				mock.EXPECT().
					GetUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(&models.User{ID: 1, Login: "user"}, nil)
			},
			wantStatus: http.StatusOK,
			wantResponse: response.AuthResponse{
				Message: "Пользователь авторизован",
				Login:   "user",
				Token:   "", // будет проверяться отдельно через checkToken
			},
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготавливаем моковое хранилище
			mockStore := mocks.NewMockStorage(ctrl)
			tt.setupMock(mockStore)

			// создаём handler с моком storage
			handler := &AppHandler{
				storage:      mockStore,
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
					assert.Equal(t, exp, got)
				case response.AuthResponse:
					// проверяем успешный ответ авторизации
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
