package netutils

import (
	"context"
	"net"
	"time"
)

const DefaultHostTimeout = 2 * time.Second

// NetworkChecker Реализация проверки доступности.
type NetworkChecker struct{}

// NewNetworkChecker Конструктор.
func NewNetworkChecker() *NetworkChecker {
	return &NetworkChecker{}
}

// IsHostReachable Проверка доступности хоста. Если timeout <= 0 - используется DefaultHostTimeout.
func (nc *NetworkChecker) IsHostReachable(ctx context.Context, address string, port string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultHostTimeout
	}

	dialer := net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, port))
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
