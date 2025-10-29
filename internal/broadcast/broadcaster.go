package broadcast

import (
	"context"
	"errors"
	"net/http"
)

//go:generate mockgen -destination=mocks/broadcast_mock.go -package=mocks . Broadcaster

var (
	ErrSubscribeNotSupported = errors.New("подписка не реализована в данном адаптере; используйте HTTPHandler()")
)

type Broadcaster interface {
	Subscribe(ctx context.Context, topic string) (<-chan []byte, func(), error)
	HTTPHandler() http.Handler
	Publish(topic string, data []byte) error
	Close() error
}
