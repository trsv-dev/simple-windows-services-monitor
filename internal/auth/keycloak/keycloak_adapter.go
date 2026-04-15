package keycloak

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
)

// KeycloakAdapter Реализует auth.AuthProvider через OIDC/Keycloak.
type KeycloakAdapter struct {
	verifier *oidc.IDTokenVerifier
}

// KeycloakConfig Конфигурация для создания адаптера.
type KeycloakConfig struct {
	IssuerURL       string
	ClientID        string
	SkipIssuerCheck bool
}

// NewKeycloakAdapter Конструктор адаптера KeycloakAdapter.
func NewKeycloakAdapter(ctx context.Context, config KeycloakConfig) (*KeycloakAdapter, error) {
	var providerCtx context.Context

	if config.SkipIssuerCheck {
		// при локальной разработке в контейнерах, когда в конфиге SkipIssuerCheck=true
		providerCtx = oidc.InsecureIssuerURLContext(ctx, config.IssuerURL)
	} else {
		providerCtx = ctx
	}

	// создаём провайдер (загружает JWKS, openid-конфиг)
	provider, err := oidc.NewProvider(providerCtx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания провайдера oidc: %w", err)
	}

	// создаем верифайер
	verifier := provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	return &KeycloakAdapter{
		verifier: verifier,
	}, nil
}

// ValidateToken Реализует интерфейс auth.AuthProvider.
func (k *KeycloakAdapter) ValidateToken(ctx context.Context, rawToken string) (*auth.UserClaims, error) {

	//верификация токена (подпись, exp, iss, aud)
	idToken, err := k.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("ошибка верификации токена: %w", err)
	}

	// парсим нужные клеймы
	var claims struct {
		Sub               string `json:"sub"`
		PreferredUsername string `json:"preferred_username"`
	}

	if err = idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("ошибка парсинга claims: %w", err)
	}

	// claims.Sub - это users.id (строка из Keycloak)
	// claims.PreferredUsername - это login для отображения
	login := claims.PreferredUsername
	id := claims.Sub

	return &auth.UserClaims{
		ID:    id,
		Login: login,
	}, nil
}
