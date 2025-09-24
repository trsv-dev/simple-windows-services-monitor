package broadcast

import (
	"context"
	"net/http"

	"github.com/r3labs/sse/v2"
)

// R3labsSSEAdapter — адаптер для библиотеки r3labs/sse.
// Обёртка предоставляет Publisher (Publish/Close) и http.Handler для монтирования.
type R3labsSSEAdapter struct {
	srv *sse.Server
}

// NewR3labsSSEAdapter создаёт новый экземпляр адаптера (и internal sse.Server).
func NewR3labsSSEAdapter() *R3labsSSEAdapter {
	srv := sse.New()

	srv.CreateStream("services")

	return &R3labsSSEAdapter{srv: srv}
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
	return a.srv
}
