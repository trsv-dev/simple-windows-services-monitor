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
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

// TestAuthMiddleware Интеграционные тесты middleware авторизации с Keycloak.
// Проверяет: извлечение токена, валидацию, создание/поиск пользователя, контекст.
func TestAuthMiddleware(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)
	mockStorage := storageMocks.NewMockStorage(ctrl)

	middleware := AuthMiddleware(mockStorage, mockAuthProvider)

	tests := []struct {
		name          string
		setupAuth     func(r *http.Request)
		setupMocks    func()
		wantStatus    int
		wantCtxLogin  string
		wantCtxUserID string
	}{
		{
			name: "успешная авторизация - пользователь существует",
			setupAuth: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer kc-valid-token")
			},
			setupMocks: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-valid-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "testuser"}, nil)
				mockStorage.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Return(&models.User{ID: "any-id-user-1", Login: "testuser"}, nil)
			},
			wantStatus:    http.StatusOK,
			wantCtxLogin:  "testuser",
			wantCtxUserID: "any-id-user-1",
		},
		{
			name: "успешная авторизация через cookie - новый пользователь создаётся",
			setupAuth: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-newuser-token"})
			},
			setupMocks: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-newuser-token").
					Return(&auth.UserClaims{ID: "any-id-user-2", Login: "newuser"}, nil)
				mockStorage.EXPECT().
					GetUser(gomock.Any(), &models.User{ID: "any-id-user-2", Login: "newuser"}).
					Return(nil, errs.NewErrUserIDNotFound(nil))
				mockStorage.EXPECT().
					CreateUser(gomock.Any(), &models.User{ID: "any-id-user-2", Login: "newuser"}).
					Return(nil)
			},
			wantStatus:    http.StatusOK,
			wantCtxLogin:  "newuser",
			wantCtxUserID: "any-id-user-2",
		},
		{
			name:      "ошибка - нет токена (нет заголовка и cookie)",
			setupAuth: func(r *http.Request) {},
			setupMocks: func() {
				// Ничего НЕ вызывается
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "ошибка - невалидный токен Keycloak",
			setupAuth: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer kc-invalid-token")
			},
			setupMocks: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-invalid-token").
					Return(nil, errors.New("oidc: token expired"))
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "создание нового пользователя (успешно)",
			setupAuth: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer kc-newuser-token")
			},
			setupMocks: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-newuser-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "newuser"}, nil)
				mockStorage.EXPECT().
					GetUser(gomock.Any(), gomock.Any()).
					Return(nil, errs.NewErrUserIDNotFound(nil))
				mockStorage.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantStatus:    http.StatusOK,
			wantCtxLogin:  "newuser",
			wantCtxUserID: "any-id-user-1",
		},
		{
			name: "ошибка БД при GetUser",
			setupAuth: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer kc-db-error-token")
			},
			setupMocks: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-db-error-token").
					Return(&auth.UserClaims{ID: "any-id-user-err", Login: "dbuser"}, nil)
				mockStorage.EXPECT().
					GetUser(gomock.Any(), &models.User{ID: "any-id-user-err", Login: "dbuser"}).
					Return(nil, errors.New("database timeout"))
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			// Next handler проверяет контекст
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				login, ok := r.Context().Value(contextkeys.Login).(string)
				if !ok || login != tt.wantCtxLogin {
					t.Errorf("ожидался login=%s, получен=%v", tt.wantCtxLogin, login)
				}

				userID, ok := r.Context().Value(contextkeys.UserID).(string)
				if !ok || userID != tt.wantCtxUserID {
					t.Errorf("ожидался userID=%s, получен=%v", tt.wantCtxUserID, userID)
				}

				w.WriteHeader(http.StatusOK)
			})

			// Применяем middleware
			handler := middleware(nextHandler)

			// Создаём запрос
			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setupAuth(r)
			w := httptest.NewRecorder()

			// Выполняем
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestAuthMiddleware_TokenSources Проверяет приоритет источников токена (Bearer > Cookie).
func TestAuthMiddleware_TokenSources(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)
	mockStorage := storageMocks.NewMockStorage(ctrl)

	middleware := AuthMiddleware(mockStorage, mockAuthProvider)

	tests := []struct {
		name       string
		authHeader string
		jwtCookie  string
		wantToken  string
		wantStatus int
	}{
		{
			name:       "Bearer имеет приоритет над Cookie",
			authHeader: "Bearer bearer-wins-token",
			jwtCookie:  "cookie-loses-token",
			wantToken:  "bearer-wins-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "только Cookie работает",
			authHeader: "",
			jwtCookie:  "only-cookie-token",
			wantToken:  "only-cookie-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "только Bearer работает",
			authHeader: "Bearer only-bearer-token",
			jwtCookie:  "",
			wantToken:  "only-bearer-token",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledToken := ""
			mockAuthProvider.EXPECT().
				ValidateToken(gomock.Any(), tt.wantToken).
				DoAndReturn(func(ctx context.Context, token string) (*auth.UserClaims, error) {
					calledToken = token
					return &auth.UserClaims{ID: "any-id-user-1", Login: "test"}, nil
				})

			mockStorage.EXPECT().
				GetUser(gomock.Any(), gomock.Any()).
				Return(&models.User{ID: "any-id-user-1", Login: "test"}, nil)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			r := httptest.NewRequest(http.MethodGet, "/test", nil)
			r.Header.Set("Authorization", tt.authHeader)
			if tt.jwtCookie != "" {
				r.AddCookie(&http.Cookie{Name: "JWT", Value: tt.jwtCookie})
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantToken, calledToken)
		})
	}
}
