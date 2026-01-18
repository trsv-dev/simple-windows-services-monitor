package broadcast

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
)

// MakeJWTTopicResolver возвращает resolver, использующий JWT из cookie "JWT".
func MakeJWTTopicResolver(JWTSecretKey string, tokenBuilder auth.TokenBuilder) TopicResolver {
	return func(r *http.Request) (string, error) {
		c, err := r.Cookie("JWT")
		if err != nil {
			return "", err
		}
		token := c.Value

		claims, err := tokenBuilder.GetClaims(token, JWTSecretKey)
		if err != nil {
			return "", err
		}
		if claims.ID <= 0 {
			return "", errors.New("неверный id пользователя")
		}

		// тип потока (servers / services)
		stream := r.URL.Query().Get("stream")
		if stream == "" {
			return "", errors.New("параметр запроса stream обязателен")
		}

		switch stream {
		case "servers", "services":
			// OK
		default:
			return "", errors.New("неизвестный тип потока")
		}

		return fmt.Sprintf("user-%d:%s", claims.ID, stream), nil
	}
}
