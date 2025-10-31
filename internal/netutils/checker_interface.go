package netutils

import "time"

//go:generate mockgen -destination=mocks/mock_network_checker.go -package=mocks . Checker

// Checker Интерфейс для проверки доступности сети.
type Checker interface {
	IsHostReachable(address string, port int, timeout time.Duration) bool
}
