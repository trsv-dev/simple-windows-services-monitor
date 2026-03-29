package session_handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/api/response"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
)

const TokenExp = time.Hour * 24

type SessionHandler struct {
	AuthProvider auth.AuthProvider
}

func NewSessionHandler(AuthProvider auth.AuthProvider) *SessionHandler {
	return &SessionHandler{AuthProvider: AuthProvider}
}

// SetSessionCookie устанавливает httpOnly куку с JWT токеном.
//
// ЗАЧЕМ ЭТО НУЖНО:
// Браузерное API EventSource (Server-Sent Events) не позволяет устанавливать
// произвольные заголовки, такие как "Authorization: Bearer <token>".
// Это ограничение самого JavaScript API - при создании EventSource нельзя добавить
// свои заголовки, браузер отправляет только стандартные (Cookie, Origin и т.д.).
//
// РЕШЕНИЕ:
// Мы используем httpOnly куку "JWT", которая автоматически прилагается ко всем
// запросам на наш домен, включая SSE-соединения, если указан флаг withCredentials: true.
// Этот хендлер вызывается фронтендом сразу после успешной аутентификации через Keycloak
// (и при каждом обновлении токена), чтобы установить эту куку.
//
// КАК ЭТО РАБОТАЕТ:
//  1. Фронтенд отправляет POST-запрос на /api/user/session с заголовком
//     "Authorization: Bearer <token>" (токен получен от Keycloak).
//  2. Мы извлекаем токен, валидируем его через AuthProvider (Keycloak).
//  3. Если токен валидный, устанавливаем httpOnly куку "JWT" с этим токеном.
//  4. Кука имеет флаги:
//     - HttpOnly: защищает от кражи через XSS (JavaScript не может прочитать куку).
//     - Secure: кука передаётся только по HTTPS (в продакшене обязательно).
//     - SameSite=Lax: защита от CSRF, но не блокирует переходы с внешних сайтов.
//     - Expires: 24 часа (можно синхронизировать с временем жизни токена).
//  5. После этого все SSE-соединения (например, /user/broadcasting?stream=servers)
//     автоматически отправляют эту куку, и сервер может аутентифицировать пользователя.
func (h *SessionHandler) SetSessionCookie(w http.ResponseWriter, r *http.Request) {
	// извлекаем токен из заголовка Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.ErrorJSON(w, http.StatusUnauthorized, "Хедер авторизации отсутствует или поврежден")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// валидируем токен (на всякий случай, хотя middleware уже проверил)
	claims, err := h.AuthProvider.ValidateToken(r.Context(), token)
	if err != nil || claims.ID == "" {
		response.ErrorJSON(w, http.StatusUnauthorized, "Невалидный токен")
		return
	}

	// устанавливаем HttpOnly куку,
	// флаг HttpOnly защищает от XSS, Secure требует HTTPS, SameSite=Lax даёт базовую защиту от CSRF.
	cookie := http.Cookie{
		Name:     "JWT",
		Value:    token,
		Expires:  time.Now().Add(TokenExp),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, &cookie)

	response.JSON(w, http.StatusOK, map[string]string{"Status": "OK"})
}
