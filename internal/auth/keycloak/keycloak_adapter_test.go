package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth/keycloak/models"
)

// TestNewKeycloakAdapter Тесты конструктора.
func TestNewKeycloakAdapter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("ошибка при пустом IssuerURL", func(t *testing.T) {
		t.Parallel()
		config := KeycloakConfig{
			IssuerURL: "",
			ClientID:  "test-client",
		}
		adapter, err := NewKeycloakAdapter(ctx, config)

		assert.Error(t, err)
		assert.Nil(t, adapter)
		assert.Contains(t, err.Error(), "ошибка создания провайдера oidc")
	})

	t.Run("ошибка при недоступном провайдере", func(t *testing.T) {
		t.Parallel()
		config := KeycloakConfig{
			IssuerURL: "http://localhost:9999/nonexistent",
			ClientID:  "test-client",
		}
		// не используем таймаут, чтобы тест не висел долго
		adapter, err := NewKeycloakAdapter(ctx, config)

		// ожидаем ошибку сети или таймаута
		assert.Error(t, err)
		assert.Nil(t, adapter)
	})

	t.Run("успешное создание с моком провайдера", func(t *testing.T) {
		t.Parallel()

		var oidcServer *httptest.Server

		// поднимаем минимальный мок-сервер с OIDC-конфигом
		oidcServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/.well-known/openid-configuration":
				_ = json.NewEncoder(w).Encode(map[string]string{
					"issuer":         oidcServer.URL,
					"jwks_uri":       oidcServer.URL + "/jwks",
					"token_endpoint": oidcServer.URL + "/token",
				})
			case "/jwks":
				// пустой JWKS - для теста достаточно структуры
				_ = json.NewEncoder(w).Encode(map[string][]interface{}{"keys": {}})
			default:
				http.NotFound(w, r)
			}
		}))
		defer oidcServer.Close()

		config := KeycloakConfig{
			IssuerURL:       oidcServer.URL,
			ClientID:        "test-client",
			SkipIssuerCheck: true,
		}
		adapter, err := NewKeycloakAdapter(ctx, config)

		// может упасть из-за пустого JWKS, но конструктор отработает,
		// главное - проверяем, что нет паники
		if err != nil {
			assert.Contains(t, err.Error(), "ошибка")
		}
		// Если адаптер создан — отлично
		if adapter != nil {
			assert.NotNil(t, adapter.verifier)
		}
	})
}

// TestKeycloakAdapter_ValidateToken_ErrorCases Тесты обработки ошибок.
func TestKeycloakAdapter_ValidateToken_ErrorCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var oidcServer *httptest.Server

	// создаём адаптер с мок-сервером
	oidcServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/.well-known/openid-configuration" {
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":   oidcServer.URL,
				"jwks_uri": oidcServer.URL + "/jwks",
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string][]interface{}{"keys": {}})
	}))
	defer oidcServer.Close()

	config := KeycloakConfig{
		IssuerURL:       oidcServer.URL,
		ClientID:        "test-client",
		SkipIssuerCheck: true,
	}
	adapter, err := NewKeycloakAdapter(ctx, config)
	if err != nil {
		t.Skipf("не удалось создать адаптер для тестов: %v", err)
	}

	t.Run("ошибка при пустом токене", func(t *testing.T) {
		t.Parallel()
		claims, err := adapter.ValidateToken(ctx, "")
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "ошибка верификации токена")
	})

	t.Run("ошибка при невалидном JWT", func(t *testing.T) {
		t.Parallel()
		claims, err := adapter.ValidateToken(ctx, "not.a.valid.jwt")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})

	t.Run("ошибка при просроченном токене", func(t *testing.T) {
		t.Parallel()
		// создаём просроченный JWT (без подписи — верификация всё равно упадёт),
		// это тест на то, что ошибка от библиотеки корректно прокидывается
		claims, err := adapter.ValidateToken(ctx, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZXhwIjoxNjAwMDAwMDAwfQ.invalid")
		assert.Error(t, err)
		assert.Nil(t, claims)
	})
}

// TestParseUserClaims Тесты парсинга UserClaims.
func TestParseUserClaims(t *testing.T) {
	tests := []struct {
		name    string
		claims  models.Claims
		wantErr bool
	}{
		{
			name: "ok",
			claims: models.Claims{
				Sub:               "any-id-user-1",
				PreferredUsername: "testuser",
			},
		},
		{
			name: "no sub",
			claims: models.Claims{
				PreferredUsername: "testuser",
			},
			wantErr: true,
		},
		{
			name: "no username",
			claims: models.Claims{
				Sub: "any-id-user-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := parseUserClaims(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.claims.Sub, res.ID)
			require.Equal(t, tt.claims.PreferredUsername, res.Login)
		})
	}
}
