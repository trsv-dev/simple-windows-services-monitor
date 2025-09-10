package service_control

import "context"

// Client Интерфейс для выполнения команд на удалённом сервере.
type Client interface {
	RunCommand(ctx context.Context, cmd string) (string, error)
}
