package service_control

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/masterzen/winrm"
)

// WinRMClient Структура WinRM клиента.
type WinRMClient struct {
	client   *winrm.Client
	endpoint *winrm.Endpoint
	user     string
	password string
}

// NewWinRMClient Конструктор, возвращающий новый WinRM клиент с нужными настройками.
func NewWinRMClient(addr, port, user, password string, useHTTPS, insecureForHTTPS bool) (*WinRMClient, error) {
	switch {
	case addr == "":
		return nil, fmt.Errorf("адрес хоста не может быть пустым")
	case port == "":
		return nil, fmt.Errorf("порт не может быть пустым")
	case user == "":
		return nil, fmt.Errorf("имя пользователя не может быть пустым")
	case password == "":
		return nil, fmt.Errorf("пароль не может быть пустым")
	}

	winrmPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("неверный порт %q: %w", port, err)
	}

	if winrmPort < 1 || winrmPort > 65535 {
		return nil, fmt.Errorf("неверный порт %q", port)
	}

	insecure := useHTTPS && insecureForHTTPS

	endpoint := &winrm.Endpoint{
		Host:     addr,
		Port:     winrmPort,
		HTTPS:    useHTTPS,
		Insecure: insecure,
		Timeout:  10 * time.Second,
	}

	newClient, err := winrm.NewClient(endpoint, user, password)
	if err != nil {
		return nil, fmt.Errorf(
			"невозможно создать клиент WinRM %s:%d (https=%t): %w",
			addr, winrmPort, useHTTPS, err,
		)
	}

	return &WinRMClient{
		client:   newClient,
		endpoint: endpoint,
		user:     user,
		password: password,
	}, nil
}

// RunCommand Выполнение команды на удаленном сервере.
func (c *WinRMClient) RunCommand(ctx context.Context, cmd string) (string, error) {
	var stdout, stderr bytes.Buffer
	_, err := c.client.RunWithContext(ctx, cmd, &stdout, &stderr)
	if err != nil {
		return "", fmt.Errorf("ошибка выполнения команды: %w; stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
