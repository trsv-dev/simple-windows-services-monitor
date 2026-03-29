package broadcast

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
)

// MakeTopicResolver возвращает resolver.
func MakeTopicResolver(authProvider auth.AuthProvider) TopicResolver {
	return func(r *http.Request) (string, error) {
		c, err := r.Cookie("JWT")
		if err != nil {
			return "", err
		}
		token := c.Value

		claims, err := authProvider.ValidateToken(r.Context(), token)
		if err != nil {
			return "", err
		}
		if claims.ID == "" {
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

		return fmt.Sprintf("user-%s:%s", claims.ID, stream), nil
	}
}
