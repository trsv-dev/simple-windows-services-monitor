package service_control

import (
	"bytes"
	"context"
	"fmt"
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
func NewWinRMClient(addr, user, password string) (*WinRMClient, error) {
	endpoint := &winrm.Endpoint{
		Host:     addr,
		Port:     5985,
		HTTPS:    false,
		Insecure: true,
		Timeout:  10 * time.Second,
	}

	newClient, err := winrm.NewClient(endpoint, user, password)
	if err != nil {
		return nil, fmt.Errorf("невозможно создать клиент WinRM: %w", err)
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
