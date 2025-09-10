package service_control

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/masterzen/winrm"
)

type WinRMClient struct {
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
		Timeout:  60 * time.Second,
	}
	return &WinRMClient{
		endpoint: endpoint,
		user:     user,
		password: password,
	}, nil
}

// RunCommand Функция запуска переданной команды на удаленном сервере.
func (c *WinRMClient) RunCommand(ctx context.Context, cmd string) (string, error) {
	client, err := winrm.NewClient(c.endpoint, c.user, c.password)
	if err != nil {
		return "", fmt.Errorf("cannot create client: %w", err)
	}

	shell, err := client.CreateShell()
	if err != nil {
		return "", fmt.Errorf("cannot create shell: %w", err)
	}
	defer shell.Close()

	command, err := shell.ExecuteWithContext(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("cannot execute command: %w", err)
	}

	// command.Stdout — это *commandReader, читаем содержимое через ReadAll
	outputBytes, err := io.ReadAll(command.Stdout)
	if err != nil {
		return "", fmt.Errorf("cannot read stdout: %w", err)
	}

	return string(outputBytes), nil
}
