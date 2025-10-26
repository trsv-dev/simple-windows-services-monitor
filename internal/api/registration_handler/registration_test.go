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
		name       string
		method     string
		body       io.Reader
		setupMock  func(m *storageMocks.MockStorage)
		wantStatus int
		checkToken bool
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
			checkToken: false,
		},
		{
			name:       "ошибка чтения тела запроса",
			method:     http.MethodPost,
			body:       &errorReader{},
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
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
			checkToken: false,
		},
		{
			name:   "логин менее 4 символов при валидации",
			method: http.MethodPost,
			body: func() io.Reader {
				u := models.User{Login: "usr", Password: "password"}
				b, _ := json.Marshal(u)
				return bytes.NewBuffer(b)
			}(),
			setupMock:  func(m *storageMocks.MockStorage) {},
			wantStatus: http.StatusBadRequest,
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
				u := models.User{Login: "user", Password: "pass"}
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
				mock.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(errs.NewErrLoginIsTaken("existing_user", errors.New("duplicate key")))
			},
			wantStatus: http.StatusConflict,
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
				mock.EXPECT().
					CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					Return(nil)
			},
			wantStatus: http.StatusCreated,
			checkToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// подготовка
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)

			// создаём handler с правильными параметрами
			handler := NewRegistrationHandler(mockStorage, "test-jwt-secret-key")

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

			// если успешная регистрация - проверяем токен
			if tt.checkToken && tt.wantStatus == http.StatusCreated {
				data, _ := io.ReadAll(res.Body)
				var got response.AuthResponse
				json.Unmarshal(data, &got)
				assert.NotEmpty(t, got.Token, "токен не должен быть пустым при успешной регистрации")
			}
		})
	}
}

// TestUserRegistrationContentType Проверяет установку заголовков при успешной регистрации.
func TestUserRegistrationContentType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// подготовка
	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockStorage.EXPECT().
		CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
		Return(nil)

	handler := NewRegistrationHandler(mockStorage, "test-jwt-secret-key")

	u := models.User{Login: "user", Password: "password"}
	b, _ := json.Marshal(u)
	r := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// выполнение регистрации
	handler.UserRegistration(w, r)

	// проверка результатов
	res := w.Result()
	defer res.Body.Close()

	// проверяем заголовки
	tests := []struct {
		name        string
		checkHeader func(t *testing.T)
	}{
		{
			name: "заголовок Content-Type установлен на application/json",
			checkHeader: func(t *testing.T) {
				assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
			},
		},
		{
			name: "статус код 201 Created",
			checkHeader: func(t *testing.T) {
				assert.Equal(t, http.StatusCreated, res.StatusCode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.checkHeader(t)
		})
	}
}

// TestUserRegistrationErrors Проверяет различные сценарии ошибок.
func TestUserRegistrationErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name       string
		body       io.Reader
		setupMock  func(m *storageMocks.MockStorage)
		wantStatus int
	}{
		{
			name: "пользователь существует",
			body: bytes.NewBufferString(`{"login":"existing","password":"password"}`),
			setupMock: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					Return(errs.NewErrLoginIsTaken("existing", errors.New("dup")))
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "ошибка БД",
			body: bytes.NewBufferString(`{"login":"new_user","password":"password"}`),
			setupMock: func(m *storageMocks.MockStorage) {
				m.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					Return(errors.New("db error"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := storageMocks.NewMockStorage(ctrl)
			tt.setupMock(mockStorage)

			handler := NewRegistrationHandler(mockStorage, "test-jwt-secret-key")

			r := httptest.NewRequest(http.MethodPost, "/register", tt.body)
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.UserRegistration(w, r)

			res := w.Result()
			defer res.Body.Close()

			assert.Equal(t, tt.wantStatus, res.StatusCode)
		})
	}
}

// TestNewRegistrationHandler Проверяет конструктор RegistrationHandler.
func TestNewRegistrationHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	jwtSecret := "test-secret-key"

	handler := NewRegistrationHandler(mockStorage, jwtSecret)

	// проверяем что все поля инициализированы
	assert.NotNil(t, handler, "handler не должен быть nil")
	assert.Equal(t, jwtSecret, handler.JWTSecretKey, "JWT ключ должен совпадать")
	assert.NotNil(t, handler.storage, "storage должно быть инициализировано")
}

// errorReader Вспомогательная структура для имитации ошибки при чтении.
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}
