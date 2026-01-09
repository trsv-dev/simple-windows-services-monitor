package netutils

import (
	"context"
	"net"
	"time"

	"github.com/prometheus-community/pro-bing"
)

const DefaultHostTimeout = 2 * time.Second

// NetworkChecker Реализация проверки доступности.
type NetworkChecker struct{}

// NewNetworkChecker Конструктор.
func NewNetworkChecker() *NetworkChecker {
	return &NetworkChecker{}
}

// CheckTCP Метод пытается установить TCP-соединение с адресом и портом в пределах
// заданного таймаута. Если соединение успешно установлено — хост считается
// доступным. Если timeout <= 0 - используется DefaultHostTimeout.
func (nc *NetworkChecker) CheckTCP(ctx context.Context, address string, port string, timeout time.Duration) bool {
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

// CheckICMP // Метод отправляет ICMP-запросы на указанный адрес и ожидает ответ
// в пределах заданного таймаута. Успешный ответ означает, что хост
// доступен на сетевом уровне. Если timeout <= 0 - используется DefaultHostTimeout.
func (nc *NetworkChecker) CheckICMP(ctx context.Context, address string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultHostTimeout
	}

	pinger, err := probing.NewPinger(address)
	if err != nil {
		return false
	}

	pinger.SetPrivileged(true)

	pinger.Count = 3
	pinger.Timeout = timeout

	// создаем канал булевых значений ёмкостью 1
	pingerDone := make(chan bool, 1)

	go func() {
		defer close(pingerDone)

		// запускаем пингер, который отправит false в канал если возникнет ошибка
		// или отправит pinger.Statistics().PacketsRecv > 0 если ошибки не будет
		pingerErr := pinger.Run()
		if pingerErr != nil {
			pingerDone <- false
			return
		}

		pingerDone <- pinger.Statistics().PacketsRecv > 0
	}()

	select {
	case <-ctx.Done():
		pinger.Stop()
		return false
	case ok := <-pingerDone: // если в пингере не произошла ошибка и кол-во отправленных пакетов больше нуля
		return ok
	}
}
