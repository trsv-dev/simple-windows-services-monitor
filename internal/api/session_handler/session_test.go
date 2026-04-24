package session_handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth/keycloak/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth/mocks"
)

// Test_SetSessionCookie_Success
// Проверяет, что при валидном токене выставляется HttpOnly кука JWT и возвращается 200 OK.
func Test_SetSessionCookie_Success(t *testing.T) {
	// создаём контроллер для gomock
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// создаём мок для AuthProvider
	mockAuth := mocks.NewMockAuthProvider(ctrl)

	// токен, который "пришёл" от клиента
	token := "valid-jwt-token"

	// ожидаем, что ValidateToken будет вызван с этим токеном и вернёт валидные claims
	mockAuth.
		EXPECT().
		ValidateToken(gomock.Any(), token).
		Return(&models.UserClaims{
			ID:    "any-id-user-1",
			Login: "testuser",
		}, nil)

	// создаём тестируемый хендлер
	h := NewSessionHandler(mockAuth)

	// собираем запрос с заголовком Authorization: Bearer <token>
	req := httptest.NewRequest(http.MethodPost, "/api/user/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// ResponseRecorder будет собирать ответ
	rr := httptest.NewRecorder()

	// вызываем хендлер
	h.SetSessionCookie(rr, req)

	// проверяем код ответа
	if rr.Code != http.StatusOK {
		t.Fatalf("ожидался статус %d, получен %d, body: %s",
			http.StatusOK, rr.Code, rr.Body.String())
	}

	// получаем полноценный http.Response из рекордера
	res := rr.Result()
	defer res.Body.Close()

	// проверяем, что Set-Cookie вообще был выставлен
	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("ожидалась хотя бы одна кука, но Set-Cookie пуст")
	}

	// ищем именно куку с именем JWT
	var jwtCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "JWT" {
			jwtCookie = c
			break
		}
	}
	if jwtCookie == nil {
		t.Fatalf("ожидалась кука с именем JWT, но её нет. Все куки: %#v", cookies)
	}

	// проверяем основные свойства куки
	if jwtCookie.Value != token {
		t.Fatalf("ожидалось значение куки %q, получено %q", token, jwtCookie.Value)
	}
	if !jwtCookie.HttpOnly {
		t.Fatalf("ожидалось, что кука будет HttpOnly")
	}
	if !jwtCookie.Secure {
		t.Fatalf("ожидалось, что кука будет Secure")
	}
	if jwtCookie.Path != "/" {
		t.Fatalf("ожидался Path=/, получен %q", jwtCookie.Path)
	}
	if jwtCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("ожидался SameSite=Lax, получен %v", jwtCookie.SameSite)
	}

	// дополнительная проверка: срок жизни куки должен быть "в будущем"
	if time.Until(jwtCookie.Expires) <= 0 {
		t.Fatalf("ожидалось, что срок жизни куки будет в будущем, Expires=%v", jwtCookie.Expires)
	}
}

// Test_SetSessionCookie_MissingAuthorization
// Проверяет, что при отсутствии или неправильном Authorization возвращается 401 и кука не ставится.
func Test_SetSessionCookie_MissingAuthorization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthProvider(ctrl)

	// в этом кейсе ValidateToken вызываться не должен
	mockAuth.EXPECT().ValidateToken(gomock.Any(), gomock.Any()).Times(0)

	h := NewSessionHandler(mockAuth)

	// запрос без заголовка Authorization
	req := httptest.NewRequest(http.MethodPost, "/api/user/session", nil)
	// req.Header.Set("Authorization", "Broken header") // можно так, результат тот же - префикс Bearer нет

	rr := httptest.NewRecorder()
	h.SetSessionCookie(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался статус %d, получен %d, body: %s",
			http.StatusUnauthorized, rr.Code, rr.Body.String())
	}

	res := rr.Result()
	defer res.Body.Close()

	// куки устанавливаться не должны
	if len(res.Cookies()) != 0 {
		t.Fatalf("куки не должны устанавливаться при неверном Authorization, но получены: %#v", res.Cookies())
	}
}

// Test_SetSessionCookie_InvalidToken_Error
// Проверяет, что при ошибке ValidateToken возвращается 401 и кука не ставится.
func Test_SetSessionCookie_InvalidToken_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthProvider(ctrl)

	token := "invalid-token"

	// имитируем ситуацию, когда провайдер авторизации не смог валидировать токен
	mockAuth.
		EXPECT().
		ValidateToken(gomock.Any(), token).
		Return(nil, errors.New("invalid token"))

	h := NewSessionHandler(mockAuth)

	req := httptest.NewRequest(http.MethodPost, "/api/user/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.SetSessionCookie(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался статус %d, получен %d, body: %s",
			http.StatusUnauthorized, rr.Code, rr.Body.String())
	}

	res := rr.Result()
	defer res.Body.Close()

	if len(res.Cookies()) != 0 {
		t.Fatalf("куки не должны устанавливаться при невалидном токене, но получены: %#v", res.Cookies())
	}
}

// Test_SetSessionCookie_InvalidToken_EmptyID
// Проверяет, что при пустом ID в claims (claims.ID == "") также возвращается 401 и кука не ставится.
func Test_SetSessionCookie_InvalidToken_EmptyID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mocks.NewMockAuthProvider(ctrl)

	token := "token-with-empty-id"

	// провайдер не возвращает ошибку, но ID в клеймах пустой — мы тоже считаем токен невалидным
	mockAuth.
		EXPECT().
		ValidateToken(gomock.Any(), token).
		Return(&models.UserClaims{
			ID:    "",
			Login: "some-login",
		}, nil)

	h := NewSessionHandler(mockAuth)

	req := httptest.NewRequest(http.MethodPost, "/api/user/session", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rr := httptest.NewRecorder()
	h.SetSessionCookie(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался статус %d, получен %d, body: %s",
			http.StatusUnauthorized, rr.Code, rr.Body.String())
	}

	res := rr.Result()
	defer res.Body.Close()

	if len(res.Cookies()) != 0 {
		t.Fatalf("куки не должны устанавливаться при пустом ID в claims, но получены: %#v", res.Cookies())
	}
}
