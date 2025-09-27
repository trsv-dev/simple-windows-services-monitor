package broadcast

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
)

// MakeJWTTopicResolver возвращает resolver, использующий JWT из cookie "JWT".
func MakeJWTTopicResolver(JWTSecretKey string) TopicResolver {
	return func(r *http.Request) (string, error) {
		c, err := r.Cookie("JWT")
		if err != nil {
			return "", err
		}
		token := c.Value

		claims, err := auth.GetClaims(token, JWTSecretKey)
		if err != nil {
			return "", err
		}
		if claims.ID <= 0 {
			return "", errors.New("неверный id пользователя")
		}

		return fmt.Sprintf("user-%d", claims.ID), nil
	}
}
