package netutils

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/prometheus-community/pro-bing"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
)

const DefaultHostTimeout = 1 * time.Second

// NetworkChecker Реализация проверки доступности.
type NetworkChecker struct{}

// NewNetworkChecker Конструктор.
func NewNetworkChecker() *NetworkChecker {
	return &NetworkChecker{}
}

// CheckWinRM CheckWinRM проверяет доступность WinRM сервиса.
// Поддерживает HTTP (5985) и HTTPS (5986).
// Если соединение успешно установлено — хост считается доступным.
// Отправляет POST запрос к /wsman и проверяет HTTP ответ.
// Если timeout <= 0, используется DefaultHostTimeout.
func (nc *NetworkChecker) CheckWinRM(ctx context.Context, address, port string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = DefaultHostTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := net.Dialer{}

	rawConn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(address, port))
	if err != nil {
		return false
	}

	var conn net.Conn = rawConn

	// гарантированно закроем итоговое соединение (используем замыкание,
	// чтобы закрыть актуальный conn при завершении).
	defer func() {
		if conn != nil {
			_ = conn.Close()
		}
	}()

	// WinRM over HTTPS (5986)
	if port == "5986" {
		tlsConn := tls.Client(rawConn, &tls.Config{
			InsecureSkipVerify: true, // допустимо для мониторинга
			ServerName:         address,
		})

		// TLS handshake должен быть под дедлайном
		if err := tlsConn.SetDeadline(time.Now().Add(timeout)); err == nil {
			if err := tlsConn.Handshake(); err != nil {
				return false
			}
		} else {
			return false
		}

		conn = tlsConn
	}

	// единый дедлайн на write + read
	_ = conn.SetDeadline(time.Now().Add(timeout))

	// host header для WinRM всегда с портом
	hostHeader := net.JoinHostPort(address, port)

	// минимальный WinRM HTTP-запрос
	req := "POST /wsman HTTP/1.1\r\n" +
		"Host: " + hostHeader + "\r\n" +
		"Content-Length: 0\r\n" +
		"Connection: close\r\n\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		return false
	}

	// читаем первую строку ответа
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	if !strings.HasPrefix(line, "HTTP/") {
		logger.Log.Debug(fmt.Sprintf("Неожиданный ответ от %s:%s: %s", address, port, line))
	}

	// WinRM всегда отвечает HTTP-статусом
	return strings.HasPrefix(line, "HTTP/")
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

	pinger.Count = 1
	pinger.Interval = 200 * time.Millisecond
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
