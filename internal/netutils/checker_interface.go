package netutils

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mocks/mock_network_checker.go -package=mocks . Checker

// Checker Интерфейс для проверки доступности сети.
type Checker interface {
	CheckTCP(ctx context.Context, address string, port string, timeout time.Duration) bool
	CheckICMP(ctx context.Context, address string, timeout time.Duration) bool
}
