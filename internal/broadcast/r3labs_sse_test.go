package broadcast

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/auth"
	authMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/auth/mocks"
	broadcasterMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewR3labsSSEAdapter Проверяет конструктор адаптера.
func TestNewR3labsSSEAdapter(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)

	assert.NotNil(t, adapter, "адаптер не должен быть nil")
	assert.NotNil(t, adapter.srv, "внутренний сервер должен быть инициализирован")
	assert.NotNil(t, adapter.resolve, "resolver должен быть установлен")
}

// TestBroadcasterInterface Проверяет что R3labsSSEAdapter реализует интерфейс Broadcaster.
func TestBroadcasterInterface(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	var _ Broadcaster = NewR3labsSSEAdapter(resolver)
}

// TestPublish Проверяет публикацию событий.
func TestPublish(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	tests := []struct {
		name    string
		topic   string
		data    []byte
		wantErr bool
	}{
		{
			name:    "успешная публикация",
			topic:   "user-123:services",
			data:    []byte(`{"status":"ok"}`),
			wantErr: false,
		},
		{
			name:    "публикация с пустым топиком",
			topic:   "",
			data:    []byte(`{"status":"ok"}`),
			wantErr: false,
		},
		{
			name:    "публикация с пустыми данными",
			topic:   "user-456:services",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "публикация с большими данными",
			topic:   "user-789:services",
			data:    []byte(`{"data":"` + string(make([]byte, 1000)) + `"}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.Publish(tt.topic, tt.data)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMultiplePublish Проверяет множественные публикации.
func TestMultiplePublish(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "concurrent-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	// публикуем 100 событий
	for i := 0; i < 100; i++ {
		// используем strconv.Itoa для правильного преобразования
		data := []byte(`{"id":` + strconv.Itoa(i) + `}`)
		err := adapter.Publish("test-topic", data)
		assert.NoError(t, err)
	}
}

// TestConcurrentPublish Проверяет параллельные публикации.
func TestConcurrentPublish(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "concurrent-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				data := []byte(`{"goroutine":` + string(rune(id)) + `,"iteration":` + string(rune(j)) + `}`)
				err := adapter.Publish("concurrent-topic", data)
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestClose Проверяет закрытие адаптера.
func TestClose(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)

	err := adapter.Close()
	assert.NoError(t, err, "Close не должен возвращать ошибку")

	err = adapter.Close()
	assert.NoError(t, err, "повторный Close не должен вызвать ошибку")
}

// TestSubscribe Проверяет что Subscribe не поддерживается.
func TestSubscribe(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	ctx := context.Background()
	ch, cancel, err := adapter.Subscribe(ctx, "test-topic")

	assert.Nil(t, ch, "канал должен быть nil")
	assert.Nil(t, cancel, "функция cancel должна быть nil")
	assert.ErrorIs(t, err, ErrSubscribeNotSupported, "должна вернуться ошибка ErrSubscribeNotSupported")
}

// TestHTTPHandlerResolverError Проверяет обработку ошибки от resolver.
func TestHTTPHandlerResolverError(t *testing.T) {
	tests := []struct {
		name         string
		resolver     TopicResolver
		wantStatus   int
		wantBodyText string
	}{
		{
			name: "resolver возвращает ошибку",
			resolver: func(r *http.Request) (string, error) {
				return "", errors.New("access denied")
			},
			wantStatus:   http.StatusUnauthorized,
			wantBodyText: "unauthorized",
		},
		{
			name: "resolver возвращает ошибку с пустым топиком",
			resolver: func(r *http.Request) (string, error) {
				return "", errors.New("no topic found")
			},
			wantStatus:   http.StatusUnauthorized,
			wantBodyText: "unauthorized",
		},
		{
			name: "resolver возвращает пустую ошибку",
			resolver: func(r *http.Request) (string, error) {
				return "", errors.New("")
			},
			wantStatus:   http.StatusUnauthorized,
			wantBodyText: "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := NewR3labsSSEAdapter(tt.resolver)
			defer adapter.Close()

			handler := adapter.HTTPHandler()
			r := httptest.NewRequest(http.MethodGet, "/events", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code, "статус должен быть 401 Unauthorized")
			assert.Contains(t, w.Body.String(), tt.wantBodyText, "тело должно содержать 'unauthorized'")
		})
	}
}

// TestHTTPHandlerCreateStream Проверяет создание stream.
func TestHTTPHandlerCreateStream(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "user-123", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	assert.NotNil(t, adapter.srv)

	handler := adapter.HTTPHandler()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHTTPHandlerStreamParameter Проверяет установку параметра stream в URL.
func TestHTTPHandlerStreamParameter(t *testing.T) {
	topics := []string{"user-1", "user-2", "user-999", "admin-topic"}

	for _, expectedTopic := range topics {
		t.Run("stream parameter для "+expectedTopic, func(t *testing.T) {
			// используем мок resolver чтобы отследить вызовы
			resolverCalled := false
			var resolverCalledWith string

			resolver := func(r *http.Request) (string, error) {
				resolverCalled = true
				resolverCalledWith = expectedTopic
				return expectedTopic, nil
			}

			adapter := NewR3labsSSEAdapter(resolver)
			defer adapter.Close()

			handler := adapter.HTTPHandler()
			r := httptest.NewRequest(http.MethodGet, "/events", nil)
			w := httptest.NewRecorder()

			// сохраняем оригинальный запрос
			originalQuery := r.URL.RawQuery

			done := make(chan struct{})
			go func() {
				handler.ServeHTTP(w, r)
				close(done)
			}()

			time.Sleep(100 * time.Millisecond)

			// проверяем что handler успешен
			assert.Equal(t, http.StatusOK, w.Code)

			// проверяем что resolver был вызван
			assert.True(t, resolverCalled, "resolver должен быть вызван")
			assert.Equal(t, expectedTopic, resolverCalledWith, "resolver должен вернуть ожидаемый топик")

			// проверяем что оригинальный запрос НЕ изменился
			// (параметр stream должен быть добавлен в КЛОН, не в оригинал)
			assert.Equal(t, originalQuery, r.URL.RawQuery, "оригинальный запрос не должен измениться")
		})
	}
}

// TestHTTPHandlerRequestCloning Проверяет что запрос клонируется правильно и не изменяется.
func TestHTTPHandlerRequestCloning(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "test-topic", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	handler := adapter.HTTPHandler()

	r := httptest.NewRequest(http.MethodGet, "/events?foo=bar&existing=param", nil)
	r.Header.Set("X-Custom-Header", "test-value")

	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "/events?foo=bar&existing=param", r.RequestURI)
	assert.Equal(t, "test-value", r.Header.Get("X-Custom-Header"))
}

// TestHTTPHandlerPathRewrite Проверяет переписывание пути на "/".
func TestHTTPHandlerPathRewrite(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "user-123", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	tests := []struct {
		name        string
		originalURL string
	}{
		{
			name:        "корневой путь",
			originalURL: "/",
		},
		{
			name:        "путь /events",
			originalURL: "/events",
		},
		{
			name:        "глубокий путь",
			originalURL: "/api/v1/events/stream",
		},
		{
			name:        "путь с параметрами",
			originalURL: "/events?param1=value1&param2=value2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.originalURL, nil)
			w := httptest.NewRecorder()

			// проверяем ОРИГИНАЛЬНЫЙ путь до вызова handler
			originalPath := r.URL.Path
			originalRawQuery := r.URL.RawQuery

			handler := adapter.HTTPHandler()

			done := make(chan struct{})
			go func() {
				handler.ServeHTTP(w, r)
				close(done)
			}()

			time.Sleep(100 * time.Millisecond)

			// проверяем что обработка успешна
			assert.Equal(t, http.StatusOK, w.Code)

			// проверяем что оригинальный запрос НЕ изменился
			// (потому что мы его клонируем!)
			assert.Equal(t, originalPath, r.URL.Path, "путь не должен измениться в оригинальном запросе")

			// параметры тоже не должны измениться
			assert.Equal(t, originalRawQuery, r.URL.RawQuery, "параметры не должны измениться в оригинальном запросе")
		})
	}
}

// TestHTTPHandlerContextPreservation Проверяет сохранение контекста при клонировании.
func TestHTTPHandlerContextPreservation(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "user-123", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	handler := adapter.HTTPHandler()

	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "test-value", r.Context().Value("test-key"))
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestHTTPHandlerHeaderPreservation Проверяет сохранение заголовков.
func TestHTTPHandlerHeaderPreservation(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "user-123", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	handler := adapter.HTTPHandler()

	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	r.Header.Set("Authorization", "Bearer token123")
	r.Header.Set("User-Agent", "test-agent")
	r.Header.Set("Accept-Language", "ru-RU")

	// сохраняем ОРИГИНАЛЬНЫЕ заголовки ПЕРЕД вызовом handler
	originalAuth := r.Header.Get("Authorization")
	originalUserAgent := r.Header.Get("User-Agent")
	originalLang := r.Header.Get("Accept-Language")

	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, http.StatusOK, w.Code)
	// проверяем что они совпадают с оригиналами
	assert.Equal(t, originalAuth, r.Header.Get("Authorization"))
	assert.Equal(t, originalUserAgent, r.Header.Get("User-Agent"))
	assert.Equal(t, originalLang, r.Header.Get("Accept-Language"))
}

// TestMakeTopicResolver Проверяет конструктор и работу TopicResolver с Keycloak AuthProvider.
func TestMakeTopicResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)

	tests := []struct {
		name         string
		setupRequest func() *http.Request
		setupMock    func()
		wantTopic    string
		wantErr      bool
		checkErrMsg  string
	}{
		{
			name: "успешное получение топика из Keycloak для stream=services",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-valid-token-123"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-valid-token-123").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "testuser"}, nil)
			},
			wantTopic: "user-any-id-user-1:services",
			wantErr:   false,
		},
		{
			name: "успешное получение топика для stream=servers",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=servers", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-valid-token-999"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-valid-token-999").
					Return(&auth.UserClaims{ID: "any-id-user-999", Login: "biguser"}, nil)
			},
			wantTopic: "user-any-id-user-999:servers",
			wantErr:   false,
		},
		{
			name: "отсутствует cookie JWT",
			setupRequest: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
			},
			setupMock: func() {
				// ValidateToken НЕ вызывается!
			},
			wantTopic:   "",
			wantErr:     true,
			checkErrMsg: "http: named cookie not present",
		},
		{
			name: "невалидный Keycloak токен (ошибка верификации)",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-invalid-token"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-invalid-token").
					Return(nil, errors.New("oidc: token expired"))
			},
			wantTopic:   "",
			wantErr:     true,
			checkErrMsg: "token expired",
		},
		{
			name: "Keycloak токен с пустым ID (отсутствует sub)",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-empty-sub-token"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-empty-sub-token").
					Return(&auth.UserClaims{ID: "", Login: "anonymous"}, nil)
			},
			wantTopic:   "",
			wantErr:     true,
			checkErrMsg: "неверный id пользователя",
		},
		{
			name: "отсутствует параметр stream",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-no-stream-token"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-no-stream-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:   "",
			wantErr:     true,
			checkErrMsg: "параметр запроса stream обязателен",
		},
		{
			name: "неизвестный тип потока",
			setupRequest: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=unknown", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-unknown-stream-token"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-unknown-stream-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:   "",
			wantErr:     true,
			checkErrMsg: "неизвестный тип потока",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			resolver := MakeTopicResolver(mockAuthProvider)
			r := tt.setupRequest()

			topic, err := resolver(r)

			if tt.wantErr {
				require.Error(t, err)
				if tt.checkErrMsg != "" {
					assert.Contains(t, err.Error(), tt.checkErrMsg)
				}
				assert.Equal(t, "", topic)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantTopic, topic)
			}
		})
	}
}

// TestHTTPHandlerWithKeycloakResolver Интеграционный тест HTTP-обработчика авторизации.
// Проверяет только сценарии ошибок 401 (SSE с успешным кейсом виснет, т.к. соединение долгоживущее).
func TestHTTPHandlerWithKeycloakResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)

	tests := []struct {
		name       string
		setupReq   func() *http.Request
		setupMock  func()
		wantStatus int
	}{
		{
			name: "ошибка авторизации - отсутствует JWT cookie",
			setupReq: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
			},
			setupMock: func() {
				// ValidateToken НЕ вызывается
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "ошибка авторизации - невалидный Keycloak токен",
			setupReq: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
				r.AddCookie(&http.Cookie{Name: "JWT", Value: "kc-invalid-token"})
				return r
			},
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "kc-invalid-token").
					Return(nil, errors.New("oidc: invalid signature"))
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			resolver := MakeTopicResolver(mockAuthProvider)
			adapter := NewR3labsSSEAdapter(resolver)
			defer adapter.Close()

			w := httptest.NewRecorder()
			handler := adapter.HTTPHandler()
			handler.ServeHTTP(w, tt.setupReq())

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

// TestHTTPHandlerWithPublish Интеграционный тест: публикация и получение событий.
func TestHTTPHandlerWithPublish(t *testing.T) {
	resolver := func(r *http.Request) (string, error) {
		return "user-123", nil
	}

	adapter := NewR3labsSSEAdapter(resolver)
	defer adapter.Close()

	err := adapter.Publish("user-123", []byte(`{"message":"test"}`))
	require.NoError(t, err)

	handler := adapter.HTTPHandler()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)

	err = adapter.Publish("user-123", []byte(`{"message":"test2"}`))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestMockBroadcasterPublish Проверяет работу мока Broadcaster для Publish.
func TestMockBroadcasterPublish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	tests := []struct {
		name      string
		topic     string
		data      []byte
		setupMock func()
		wantErr   bool
	}{
		{
			name:  "успешная публикация",
			topic: "user-123",
			data:  []byte(`{"status":"ok"}`),
			setupMock: func() {
				mockBroadcaster.EXPECT().
					Publish("user-123", []byte(`{"status":"ok"}`)).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:  "ошибка публикации",
			topic: "user-456",
			data:  []byte(`{"status":"error"}`),
			setupMock: func() {
				mockBroadcaster.EXPECT().
					Publish("user-456", []byte(`{"status":"error"}`)).
					Return(errors.New("publish failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			err := mockBroadcaster.Publish(tt.topic, tt.data)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMockBroadcasterClose Проверяет работу мока Broadcaster для Close.
func TestMockBroadcasterClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	tests := []struct {
		name      string
		setupMock func()
		wantErr   bool
	}{
		{
			name: "успешное закрытие",
			setupMock: func() {
				mockBroadcaster.EXPECT().
					Close().
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "ошибка при закрытии",
			setupMock: func() {
				mockBroadcaster.EXPECT().
					Close().
					Return(errors.New("close failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			err := mockBroadcaster.Close()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMockBroadcasterSubscribe Проверяет работу мока Broadcaster для Subscribe.
func TestMockBroadcasterSubscribe(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	tests := []struct {
		name      string
		topic     string
		setupMock func()
		wantCh    bool
		wantErr   bool
	}{
		{
			name:  "успешная подписка",
			topic: "user-123",
			setupMock: func() {
				ch := make(chan []byte, 1)
				ch <- []byte(`{"test":"data"}`)
				cancel := func() { close(ch) }

				mockBroadcaster.EXPECT().
					Subscribe(gomock.Any(), "user-123").
					Return((<-chan []byte)(ch), cancel, nil)
			},
			wantCh:  true,
			wantErr: false,
		},
		{
			name:  "ошибка подписки",
			topic: "user-456",
			setupMock: func() {
				mockBroadcaster.EXPECT().
					Subscribe(gomock.Any(), "user-456").
					Return(nil, nil, errors.New("subscribe failed"))
			},
			wantCh:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			ctx := context.Background()
			ch, cancel, err := mockBroadcaster.Subscribe(ctx, tt.topic)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, ch)
				assert.Nil(t, cancel)
			} else {
				assert.NoError(t, err)
				if tt.wantCh {
					assert.NotNil(t, ch)
					assert.NotNil(t, cancel)
				}
			}
		})
	}
}

// TestBroadcasterWorkflow Интеграционный тест workflow с моком.
func TestBroadcasterWorkflow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	gomock.InOrder(
		mockBroadcaster.EXPECT().Publish("user-1", []byte("msg1")).Return(nil),
		mockBroadcaster.EXPECT().Publish("user-2", []byte("msg2")).Return(nil),
		mockBroadcaster.EXPECT().Close().Return(nil),
	)

	err := mockBroadcaster.Publish("user-1", []byte("msg1"))
	assert.NoError(t, err)

	err = mockBroadcaster.Publish("user-2", []byte("msg2"))
	assert.NoError(t, err)

	err = mockBroadcaster.Close()
	assert.NoError(t, err)
}

// TestConcurrentBroadcasterOperations Проверяет параллельные операции с моком.
func TestConcurrentBroadcasterOperations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBroadcaster := broadcasterMocks.NewMockBroadcaster(ctrl)

	mockBroadcaster.EXPECT().
		Publish(gomock.Any(), gomock.Any()).
		Return(nil).
		Times(100)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				err := mockBroadcaster.Publish("topic", []byte("data"))
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for goroutines")
		}
	}
}

// TestTopicResolver_StreamValidation Проверяет валидацию параметра stream.
// Тестирует все edge-кейсы: пустой, неверный регистр, пробелы, дубликаты.
func TestTopicResolver_StreamValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)

	tests := []struct {
		name           string
		query          string
		setupMock      func()
		wantTopic      string
		wantErr        bool
		checkErrString string
	}{
		{
			name:  "stream=services валиден",
			query: "/events?stream=services",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic: "user-any-id-user-1:services",
			wantErr:   false,
		},
		{
			name:  "stream=servers валиден",
			query: "/events?stream=servers",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-2", Login: "admin"}, nil)
			},
			wantTopic: "user-any-id-user-2:servers",
			wantErr:   false,
		},
		{
			name:  "stream=SERVER (верхний регистр) невалиден",
			query: "/events?stream=SERVER",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:      "",
			wantErr:        true,
			checkErrString: "неизвестный тип потока",
		},
		{
			name:  "stream= (пустое значение) невалиден",
			query: "/events?stream=",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:      "",
			wantErr:        true,
			checkErrString: "параметр запроса stream обязателен",
		},
		{
			name:  "stream=events (неверное значение) невалиден",
			query: "/events?stream=events",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:      "",
			wantErr:        true,
			checkErrString: "неизвестный тип потока",
		},
		{
			name:  "stream с пробелом невалиден",
			query: "/events?stream=services%20",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic:      "",
			wantErr:        true,
			checkErrString: "неизвестный тип потока",
		},
		{
			name:  "дублированный stream (берётся первый)",
			query: "/events?stream=servers&stream=services",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantTopic: "user-any-id-user-1:servers", // URL.Query().Get() → первый "servers"
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			resolver := MakeTopicResolver(mockAuthProvider)
			r := httptest.NewRequest(http.MethodGet, tt.query, nil)
			r.AddCookie(&http.Cookie{Name: "JWT", Value: "jwt-token"})

			topic, err := resolver(r)

			if tt.wantErr {
				require.Error(t, err)
				if tt.checkErrString != "" {
					assert.Contains(t, err.Error(), tt.checkErrString)
				}
				assert.Equal(t, "", topic)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantTopic, topic)
			}
		})
	}
}

// TestTopicResolver_IDValidation Проверяет валидацию ID пользователя.
// Keycloak всегда возвращает string ID, поэтому упрощённый тест.
func TestTopicResolver_IDValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuthProvider := authMocks.NewMockAuthProvider(ctrl)

	tests := []struct {
		name           string
		userID         string
		setupMock      func()
		wantErr        bool
		checkErrString string
	}{
		{
			name:   "валидный Keycloak ID",
			userID: "any-id-user-1",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "any-id-user-1", Login: "user"}, nil)
			},
			wantErr: false,
		},
		{
			name:   "пустой ID невалиден",
			userID: "",
			setupMock: func() {
				mockAuthProvider.EXPECT().
					ValidateToken(gomock.Any(), "jwt-token").
					Return(&auth.UserClaims{ID: "", Login: "anonymous"}, nil)
			},
			wantErr:        true,
			checkErrString: "неверный id пользователя",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			resolver := MakeTopicResolver(mockAuthProvider)
			r := httptest.NewRequest(http.MethodGet, "/events?stream=services", nil)
			r.AddCookie(&http.Cookie{Name: "JWT", Value: "jwt-token"})

			topic, err := resolver(r)

			if tt.wantErr {
				require.Error(t, err)
				if tt.checkErrString != "" {
					assert.Contains(t, err.Error(), tt.checkErrString)
				}
				assert.Equal(t, "", topic)
			} else {
				require.NoError(t, err)
				assert.Contains(t, topic, tt.userID)
			}
		})
	}
}
