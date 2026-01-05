package netutils

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mocks/mock_network_checker.go -package=mocks . Checker

// Checker Интерфейс для проверки доступности сети.
type Checker interface {
	IsHostReachable(ctx context.Context, address string, port string, timeout time.Duration) bool
}
