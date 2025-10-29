package broadcast

import (
	"context"
	"fmt"
	"net/http"

	"github.com/r3labs/sse/v2"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

// TopicResolver Из запроса возвращает разрешённый топик (например "user-123")
type TopicResolver func(r *http.Request) (string, error)

// R3labsSSEAdapter — адаптер для библиотеки r3labs/sse.
// Обёртка предоставляет Publisher (Publish/Close) и http.Handler для монтирования.
type R3labsSSEAdapter struct {
	srv     *sse.Server
	resolve TopicResolver
}

// NewR3labsSSEAdapter Создаёт новый экземпляр адаптера (и internal sse.Server).
func NewR3labsSSEAdapter(resolve TopicResolver) *R3labsSSEAdapter {
	srv := sse.New()

	return &R3labsSSEAdapter{srv: srv, resolve: resolve}
}

// Publish реализует интерфейс Publisher.
// Публикует событие в указанный топик (stream). Данные передаются в поле Event.Data.
func (a *R3labsSSEAdapter) Publish(topic string, data []byte) error {
	a.srv.Publish(topic, &sse.Event{Data: data})
	return nil
}

// Close Закрывает все EventSource соединения.
func (a *R3labsSSEAdapter) Close() error {
	a.srv.Close() // закрывает все EventSource соединения
	return nil
}

// Subscribe r3labs реализует подписки по HTTP, а не через Go-каналы.
// Поэтому вызов Subscribe на этом адаптере не поддерживается и возвращает ErrSubscribeNotSupported.
// Используй HTTPHandler() для обслуживания подключений EventSource.
func (a *R3labsSSEAdapter) Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error) {
	return nil, nil, ErrSubscribeNotSupported
}

// HTTPHandler возвращает http.Handler, который можно примонтировать в маршруты (например, на /events/).
// r3labs.Server сам обрабатывает URL вида /<stream> и управляет подписками/реплеем.
func (a *R3labsSSEAdapter) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// аутентификация и получение топика
		topic, err := a.resolve(r)
		if err != nil {
			logger.Log.Error("SSE: топик не разрешён", logger.String("err", err.Error()))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// создаём stream заранее (устраняет возможные ошибки при подключении)
		a.srv.CreateStream(topic)

		// защитный recover вокруг ServeHTTP, чтобы не падать в случае паники внутри библиотеки
		defer func() {
			if rec := recover(); rec != nil {
				logger.Log.Error("SSE: panic recovered in handler", logger.String("panic", fmt.Sprintf("%v", rec)))
				// не перезагружаем процесс здесь — просто корректно вернём 500
			}
		}()

		// клонируем запрос
		r2 := r.Clone(r.Context())

		// формируем корректный URL с параметром stream
		q := r2.URL.Query()
		q.Set("stream", topic)
		r2.URL.RawQuery = q.Encode()

		// сбрасываем путь на корень для r3labs
		r2.URL.Path = "/"

		// передаём в r3labs/sse
		a.srv.ServeHTTP(w, r2)
	})
}
