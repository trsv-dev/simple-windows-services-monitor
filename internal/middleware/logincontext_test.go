package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	authMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/auth/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestUserLoginUserIdToContextMiddlewareSuccess Проверяет успешное добавление логина и ID в контекст.
func TestUserLoginUserIdToContextMiddlewareSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// настраиваем мок
	mockTokenBuilder.EXPECT().
		GetClaims("valid-token", secretKey).
		Return(&auth.Claims{ID: 123, Login: "testuser"}, nil)

	// флаги для проверки
	var capturedLogin string
	var capturedID int64
	nextCalled := false

	// создаём next handler
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		// проверяем что значения в контексте
		login, ok := r.Context().Value(contextkeys.Login).(string)
		assert.True(t, ok, "логин должен быть в контексте")
		capturedLogin = login

		id, ok := r.Context().Value(contextkeys.ID).(int64)
		assert.True(t, ok, "ID должен быть в контексте")
		capturedID = id

		w.WriteHeader(http.StatusOK)
	})

	// оборачиваем в middleware
	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// создаём запрос с JWT cookie
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "valid-token"})
	w := httptest.NewRecorder()

	// вызываем handler
	handler.ServeHTTP(w, r)

	// проверяем что next handler был вызван
	assert.True(t, nextCalled, "next handler должен быть вызван")

	// проверяем что значения правильные
	assert.Equal(t, "testuser", capturedLogin)
	assert.Equal(t, int64(123), capturedID)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUserLoginUserIdToContextMiddlewareNoCookie Проверяет запрос без JWT cookie.
func TestUserLoginUserIdToContextMiddlewareNoCookie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// GetClaims НЕ должен быть вызван, так как cookie отсутствует
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// создаём запрос БЕЗ JWT cookie
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// вызываем handler
	handler.ServeHTTP(w, r)

	// проверяем что next handler НЕ был вызван
	assert.False(t, nextCalled, "next handler НЕ должен быть вызван без cookie")

	// проверяем статус
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// проверяем что ответ содержит сообщение об ошибке
	assert.Contains(t, w.Body.String(), "не аутентифицирован")
}

// TestUserLoginUserIdToContextMiddlewareInvalidToken Проверяет запрос с невалидным токеном.
func TestUserLoginUserIdToContextMiddlewareInvalidToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// настраиваем мок на возврат ошибки
	mockTokenBuilder.EXPECT().
		GetClaims("invalid-token", secretKey).
		Return(nil, errors.New("invalid token"))

	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// создаём запрос с невалидным токеном
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "invalid-token"})
	w := httptest.NewRecorder()

	// вызываем handler
	handler.ServeHTTP(w, r)

	// проверяем что next handler НЕ был вызван
	assert.False(t, nextCalled, "next handler НЕ должен быть вызван с невалидным токеном")

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// проверяем что ответ содержит сообщение об ошибке
	assert.Contains(t, w.Body.String(), "идентификации")
}

// TestUserLoginUserIdToContextMiddlewareDifferentUsers Проверяет разных пользователей.
func TestUserLoginUserIdToContextMiddlewareDifferentUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	tests := []struct {
		name          string
		token         string
		expectedID    int64
		expectedLogin string
	}{
		{
			name:          "пользователь alice",
			token:         "token-alice",
			expectedID:    1,
			expectedLogin: "alice",
		},
		{
			name:          "пользователь bob",
			token:         "token-bob",
			expectedID:    2,
			expectedLogin: "bob",
		},
		{
			name:          "пользователь admin",
			token:         "token-admin",
			expectedID:    999,
			expectedLogin: "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// настраиваем мок для этого теста
			mockTokenBuilder.EXPECT().
				GetClaims(tt.token, secretKey).
				Return(&auth.Claims{ID: tt.expectedID, Login: tt.expectedLogin}, nil)

			var capturedLogin string
			var capturedID int64

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				login := r.Context().Value(contextkeys.Login).(string)
				id := r.Context().Value(contextkeys.ID).(int64)

				capturedLogin = login
				capturedID = id

				w.WriteHeader(http.StatusOK)
			})

			middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
			handler := middleware(nextHandler)

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.AddCookie(&http.Cookie{Name: "JWT", Value: tt.token})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем что значения правильные для каждого пользователя
			assert.Equal(t, tt.expectedLogin, capturedLogin)
			assert.Equal(t, tt.expectedID, capturedID)
		})
	}
}

// TestUserLoginUserIdToContextMiddlewareContextNotModified Проверяет что контекст правильно добавляет значения.
func TestUserLoginUserIdToContextMiddlewareContextNotModified(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	mockTokenBuilder.EXPECT().
		GetClaims("token", secretKey).
		Return(&auth.Claims{ID: 123, Login: "user"}, nil)

	// флаги для проверки
	var capturedLogin string
	var capturedID int64
	var contextInHandler context.Context

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// сохраняем контекст из handler'а
		contextInHandler = r.Context()

		// проверяем что значения есть в контексте
		login, ok := r.Context().Value(contextkeys.Login).(string)
		assert.True(t, ok, "логин должен быть в контексте")
		capturedLogin = login

		id, ok := r.Context().Value(contextkeys.ID).(int64)
		assert.True(t, ok, "ID должен быть в контексте")
		capturedID = id

		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// создаём запрос
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})

	// сохраняем оригинальный контекст
	originalCtx := r.Context()

	w := httptest.NewRecorder()

	// вызываем handler
	handler.ServeHTTP(w, r)

	// проверяем что контекст ВНУТРИ handler'а отличается от оригинального
	// (потому что middleware добавил значения)
	assert.NotEqual(t, originalCtx, contextInHandler, "контекст в handler'е должен отличаться от оригинального")

	// проверяем что значения правильно добавлены
	assert.Equal(t, "user", capturedLogin)
	assert.Equal(t, int64(123), capturedID)

	// проверяем что оригинальный контекст ВСЕ ЕЩЕ пустой (не имеет значений)
	_, ok := originalCtx.Value(contextkeys.Login).(string)
	assert.False(t, ok, "оригинальный контекст НЕ должен иметь логин")

	_, ok = originalCtx.Value(contextkeys.ID).(int64)
	assert.False(t, ok, "оригинальный контекст НЕ должен иметь ID")
}

// TestUserLoginUserIdToContextMiddlewareEmptyToken Проверяет запрос с пустым токеном.
func TestUserLoginUserIdToContextMiddlewareEmptyToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// настраиваем мок на пустой токен
	mockTokenBuilder.EXPECT().
		GetClaims("", secretKey).
		Return(nil, errors.New("empty token"))

	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// создаём запрос с пустым токеном
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: ""})
	w := httptest.NewRecorder()

	// вызываем handler
	handler.ServeHTTP(w, r)

	// проверяем что next handler НЕ был вызван
	assert.False(t, nextCalled)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestUserLoginUserIdToContextMiddlewareContextValues Проверяет что значения правильно добавлены в контекст.
func TestUserLoginUserIdToContextMiddlewareContextValues(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	mockTokenBuilder.EXPECT().
		GetClaims("token", secretKey).
		Return(&auth.Claims{ID: 456, Login: "testuser"}, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// проверяем что оба значения есть в контексте
		login, loginOk := r.Context().Value(contextkeys.Login).(string)
		id, idOk := r.Context().Value(contextkeys.ID).(int64)

		// убеждаемся что оба типа проверены правильно
		assert.True(t, loginOk, "логин должен быть string")
		assert.True(t, idOk, "ID должен быть int64")

		// проверяем значения
		assert.Equal(t, "testuser", login)
		assert.Equal(t, int64(456), id)

		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUserLoginUserIdToContextMiddlewareMultipleCalls Проверяет множественные вызовы middleware.
func TestUserLoginUserIdToContextMiddlewareMultipleCalls(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// ожидаем 3 вызова GetClaims
	mockTokenBuilder.EXPECT().
		GetClaims(gomock.Any(), secretKey).
		Return(&auth.Claims{ID: 1, Login: "user"}, nil).
		Times(3)

	callCount := 0

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	// делаем 3 запроса
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusOK, w.Code)
	}

	// проверяем что next handler был вызван 3 раза
	assert.Equal(t, 3, callCount)
}

// TestUserLoginUserIdToContextMiddlewareDifferentSecretKeys Проверяет разные secret keys.
func TestUserLoginUserIdToContextMiddlewareDifferentSecretKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)

	// первый вызов с первым secret key
	mockTokenBuilder.EXPECT().
		GetClaims("token1", "secret1").
		Return(&auth.Claims{ID: 1, Login: "user1"}, nil)

	// второй вызов со вторым secret key
	mockTokenBuilder.EXPECT().
		GetClaims("token2", "secret2").
		Return(&auth.Claims{ID: 2, Login: "user2"}, nil)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// создаём два разных middleware с разными secret keys
	middleware1 := UserLoginUserIdToContextMiddleware("secret1", mockTokenBuilder)
	middleware2 := UserLoginUserIdToContextMiddleware("secret2", mockTokenBuilder)

	handler1 := middleware1(nextHandler)
	handler2 := middleware2(nextHandler)

	// первый запрос
	r1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	r1.AddCookie(&http.Cookie{Name: "JWT", Value: "token1"})
	w1 := httptest.NewRecorder()

	handler1.ServeHTTP(w1, r1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// второй запрос
	r2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	r2.AddCookie(&http.Cookie{Name: "JWT", Value: "token2"})
	w2 := httptest.NewRecorder()

	handler2.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

// TestUserLoginUserIdToContextMiddlewareLargeID Проверяет работу с большим ID.
func TestUserLoginUserIdToContextMiddlewareLargeID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// используем большое ID значение
	largeID := int64(9223372036854775807) // max int64

	mockTokenBuilder.EXPECT().
		GetClaims("token", secretKey).
		Return(&auth.Claims{ID: largeID, Login: "user"}, nil)

	var capturedID int64

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(contextkeys.ID).(int64)
		capturedID = id
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что большое ID сохранилось
	assert.Equal(t, largeID, capturedID)
}

// TestUserLoginUserIdToContextMiddlewareLongLogin Проверяет работу с длинным логином.
func TestUserLoginUserIdToContextMiddlewareLongLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	// создаём длинный логин
	longLogin := string(make([]byte, 1000))
	for range longLogin {
		longLogin = "a"
	}

	mockTokenBuilder.EXPECT().
		GetClaims("token", secretKey).
		Return(&auth.Claims{ID: 1, Login: longLogin}, nil)

	var capturedLogin string

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := r.Context().Value(contextkeys.Login).(string)
		capturedLogin = login
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
	handler := middleware(nextHandler)

	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// проверяем что длинный логин сохранился
	assert.Equal(t, longLogin, capturedLogin)
}

// TestUserLoginUserIdToContextMiddlewareDifferentHTTPMethods Проверяет разные HTTP методы.
func TestUserLoginUserIdToContextMiddlewareDifferentHTTPMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTokenBuilder := authMocks.NewMockTokenBuilder(ctrl)
	secretKey := "test-secret-key"

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			mockTokenBuilder.EXPECT().
				GetClaims("token", secretKey).
				Return(&auth.Claims{ID: 1, Login: "user"}, nil)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := UserLoginUserIdToContextMiddleware(secretKey, mockTokenBuilder)
			handler := middleware(nextHandler)

			r := httptest.NewRequest(method, "/test", nil)
			r.AddCookie(&http.Cookie{Name: "JWT", Value: "token"})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			// проверяем что все методы работают
			assert.Equal(t, http.StatusOK, w.Code, "метод %s должен работать", method)
		})
	}
}
