package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// TestNewJWTTokenBuilder Проверяет конструктор JWTTokenBuilder.
func TestNewJWTTokenBuilder(t *testing.T) {
	builder := NewJWTTokenBuilder()

	assert.NotNil(t, builder, "JWTTokenBuilder не должен быть nil")
}

// TestBuildJWTToken Проверяет создание JWT-токена.
func TestBuildJWTToken(t *testing.T) {
	builder := JWTTokenBuilder{}
	secretKey := "test-secret-key"

	tests := []struct {
		name       string
		user       *models.User
		secretKey  string
		wantErr    bool
		validateFn func(t *testing.T, tokenString string)
	}{
		{
			name: "успешное создание токена",
			user: &models.User{
				ID:    123,
				Login: "testuser",
			},
			secretKey: secretKey,
			wantErr:   false,
			validateFn: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString, "токен не должен быть пустым")

				// парсим токен для проверки содержимого
				claims := &Claims{}
				token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil
				})

				require.NoError(t, err)
				assert.True(t, token.Valid)
				assert.Equal(t, int64(123), claims.ID)
				assert.Equal(t, "testuser", claims.Login)
			},
		},
		{
			name: "создание токена с пустым логином",
			user: &models.User{
				ID:    456,
				Login: "",
			},
			secretKey: secretKey,
			wantErr:   false,
			validateFn: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString)

				claims := &Claims{}
				_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil
				})

				require.NoError(t, err)
				assert.Equal(t, int64(456), claims.ID)
				assert.Empty(t, claims.Login)
			},
		},
		{
			name: "создание токена с нулевым ID",
			user: &models.User{
				ID:    0,
				Login: "zerouser",
			},
			secretKey: secretKey,
			wantErr:   false,
			validateFn: func(t *testing.T, tokenString string) {
				assert.NotEmpty(t, tokenString)

				claims := &Claims{}
				_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil
				})

				require.NoError(t, err)
				assert.Equal(t, int64(0), claims.ID)
				assert.Equal(t, "zerouser", claims.Login)
			},
		},
		{
			name: "проверка времени истечения токена",
			user: &models.User{
				ID:    789,
				Login: "expuser",
			},
			secretKey: secretKey,
			wantErr:   false,
			validateFn: func(t *testing.T, tokenString string) {
				claims := &Claims{}
				_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
					return []byte(secretKey), nil
				})

				require.NoError(t, err)

				// проверяем что время истечения установлено корректно (примерно через 24 часа)
				expectedExpiry := time.Now().Add(TokenExp)
				actualExpiry := claims.ExpiresAt.Time

				// допуск в 5 секунд
				assert.WithinDuration(t, expectedExpiry, actualExpiry, 5*time.Second)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := builder.BuildJWTToken(tt.user, tt.secretKey)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.validateFn != nil {
				tt.validateFn(t, tokenString)
			}
		})
	}
}

// TestGetClaims Проверяет получение claims из токена.
func TestGetClaims(t *testing.T) {
	builder := JWTTokenBuilder{}
	secretKey := "test-secret-key"

	// создаём валидный токен для тестов
	user := &models.User{ID: 123, Login: "testuser"}
	validToken, err := builder.BuildJWTToken(user, secretKey)
	require.NoError(t, err)

	tests := []struct {
		name        string
		tokenString string
		secretKey   string
		wantErr     bool
		wantClaims  *Claims
	}{
		{
			name:        "успешное получение claims",
			tokenString: validToken,
			secretKey:   secretKey,
			wantErr:     false,
			wantClaims: &Claims{
				Login: "testuser",
				ID:    123,
			},
		},
		{
			name:        "неверный секретный ключ",
			tokenString: validToken,
			secretKey:   "wrong-secret",
			wantErr:     true,
			wantClaims:  nil,
		},
		{
			name:        "невалидный токен",
			tokenString: "invalid.token.string",
			secretKey:   secretKey,
			wantErr:     true,
			wantClaims:  nil,
		},
		{
			name:        "пустой токен",
			tokenString: "",
			secretKey:   secretKey,
			wantErr:     true,
			wantClaims:  nil,
		},
		{
			name: "токен с неверным методом подписи",
			tokenString: func() string {
				// создаём токен с RS256 вместо HS256
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
					},
					Login: "testuser",
					ID:    123,
				}
				token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
				// попытка подписать (не сработает, но для теста достаточно)
				tokenString, _ := token.SigningString()
				return tokenString
			}(),
			secretKey:  secretKey,
			wantErr:    true,
			wantClaims: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := builder.GetClaims(tt.tokenString, tt.secretKey)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, claims)
			assert.Equal(t, tt.wantClaims.Login, claims.Login)
			assert.Equal(t, tt.wantClaims.ID, claims.ID)
		})
	}
}

// TestGetClaimsExpiredToken Проверяет работу с истёкшим токеном.
func TestGetClaimsExpiredToken(t *testing.T) {
	builder := JWTTokenBuilder{}
	secretKey := "test-secret-key"

	// создаём токен с истёкшим сроком действия
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)), // истёк час назад
		},
		Login: "expireduser",
		ID:    999,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	expiredToken, err := token.SignedString([]byte(secretKey))
	require.NoError(t, err)

	// пытаемся получить claims из истёкшего токена
	_, err = builder.GetClaims(expiredToken, secretKey)

	assert.Error(t, err, "должна быть ошибка для истёкшего токена")
}

// TestCreateCookie Проверяет создание cookie с JWT-токеном.
func TestCreateCookie(t *testing.T) {
	tests := []struct {
		name        string
		tokenString string
		validate    func(t *testing.T, cookie *http.Cookie)
	}{
		{
			name:        "создание cookie с токеном",
			tokenString: "test-jwt-token-string",
			validate: func(t *testing.T, cookie *http.Cookie) {
				assert.Equal(t, "JWT", cookie.Name, "имя cookie должно быть JWT")
				assert.Equal(t, "test-jwt-token-string", cookie.Value, "значение cookie должно быть токеном")
				assert.Equal(t, "/", cookie.Path, "path должен быть /")

				// проверяем что время истечения установлено (примерно через 24 часа)
				expectedExpiry := time.Now().Add(TokenExp)
				assert.WithinDuration(t, expectedExpiry, cookie.Expires, 5*time.Second)
			},
		},
		{
			name:        "создание cookie с пустым токеном",
			tokenString: "",
			validate: func(t *testing.T, cookie *http.Cookie) {
				assert.Equal(t, "JWT", cookie.Name)
				assert.Empty(t, cookie.Value)
			},
		},
		{
			name:        "создание cookie с длинным токеном",
			tokenString: "very.long.jwt.token.string.with.many.characters.and.dots",
			validate: func(t *testing.T, cookie *http.Cookie) {
				assert.Equal(t, "JWT", cookie.Name)
				assert.Equal(t, "very.long.jwt.token.string.with.many.characters.and.dots", cookie.Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			CreateCookie(w, tt.tokenString)

			// получаем установленные cookies
			result := w.Result()
			cookies := result.Cookies()

			require.Len(t, cookies, 1, "должна быть установлена одна cookie")

			if tt.validate != nil {
				tt.validate(t, cookies[0])
			}
		})
	}
}

// TestBuildAndGetClaimsIntegration Интеграционный тест: создание и парсинг токена.
func TestBuildAndGetClaimsIntegration(t *testing.T) {
	builder := JWTTokenBuilder{}
	secretKey := "integration-test-secret"

	user := &models.User{
		ID:    777,
		Login: "integrationuser",
	}

	// создаём токен
	tokenString, err := builder.BuildJWTToken(user, secretKey)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// парсим токен
	claims, err := builder.GetClaims(tokenString, secretKey)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// проверяем что данные совпадают
	assert.Equal(t, user.ID, claims.ID)
	assert.Equal(t, user.Login, claims.Login)
}
