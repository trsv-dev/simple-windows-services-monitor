package netutils

import (
	"fmt"
	"net"
	"time"
)

const DefaultHostTimeout = 2 * time.Second

// IsHostReachable Проверка доступности хоста. Если timeout <= 0 - используется DefaultHostTimeout.
func IsHostReachable(address string, port int, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultHostTimeout
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(address, fmt.Sprintf("%d", port)), timeout)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
