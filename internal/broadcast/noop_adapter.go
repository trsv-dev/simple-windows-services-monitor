package broadcast

import (
	"context"
	"net/http"
)

// NoopAdapter — заглушка для режима без web-интерфейса.
// Реализует те же методы, что и R3labsSSEAdapter (интерфейс Broadcaster), но не делает ничего.
type NoopAdapter struct {
	resolve TopicResolver
}

// NewNoopAdapter создаёт новый экземпляр "пустого" адаптера.
// Можно передавать резольвер, чтобы интерфейс совпадал.
func NewNoopAdapter(resolve TopicResolver) *NoopAdapter {
	return &NoopAdapter{resolve: resolve}
}

// Publish ничего не делает и всегда возвращает nil.
func (n *NoopAdapter) Publish(topic string, data []byte) error {
	return nil
}

// Close ничего не делает и всегда возвращает nil.
func (n *NoopAdapter) Close() error {
	return nil
}

// Subscribe возвращает nil-канал и nil-отписку.
func (n *NoopAdapter) Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error) {
	return nil, func() {}, nil
}

// HTTPHandler возвращает http.Handler, который просто отвечает 404 Not Found.
// Таким образом, если кто-то попытается подключиться к /events/, получит понятный ответ.
func (n *NoopAdapter) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
