package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

type Claims struct {
	jwt.RegisteredClaims
	Login string
	ID    int64
}

const TokenExp = time.Hour * 24

// BuildJWTToken Создание JWT-токена.
func BuildJWTToken(user *models.User, JWTSecretKey string) (string, error) {
	// создаем экземпляр структуры, которую будем записывать в токен
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(TokenExp)),
		},
		Login: user.Login,
		ID:    user.ID,
	}

	// создаем токен с claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// подписываем секретным ключом и возвращаем токен в виде строки
	tokenString, err := token.SignedString([]byte(JWTSecretKey))
	if err != nil {
		return "", fmt.Errorf("не удалось подписать токен: %w", err)
	}

	return tokenString, nil
}

// GetClaims Получение Claims (login и id) пользователя с помощью распарсивания JWT-токена.
func GetClaims(tokenString, JWTSecretKey string) (*Claims, error) {
	// создаем пустой экземпляр Claims, куда будем распарсивать токен
	claims := &Claims{}

	// распарсиваем токен, проверяя на метод подписи
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return "", fmt.Errorf("неверный метод подписи: %v", t.Header["alg"])
		}

		return []byte(JWTSecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга токена: %w", err)
	}

	// проверяем токен на валидность
	if !token.Valid {
		return nil, fmt.Errorf("токен недействителен")
	}

	// возвращаем значение экземпляра структуры Claims
	return claims, nil
}

// CreateCookie Создание и установка куки с JWT-токеном.
func CreateCookie(w http.ResponseWriter, tokenString string) {
	cookie := http.Cookie{
		Name:    "JWT",
		Value:   tokenString,
		Expires: time.Now().Add(TokenExp),
		Path:    "/",
	}

	http.SetCookie(w, &cookie)
}
