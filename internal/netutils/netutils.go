package netutils

import (
	"fmt"
	"net"
	"time"
)

// IsHostReachable Проверка доступности хоста
func IsHostReachable(address string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(address, fmt.Sprintf("%d", port)), timeout)
	if err != nil {
		return false
	}

	_ = conn.Close()

	return true
}
