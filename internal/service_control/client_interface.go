package service_control

import "context"

//go:generate mockgen -destination=mocks/mock_client.go -package=mocks . Client

// Client Интерфейс для выполнения команд на удалённом сервере.
type Client interface {
	RunCommand(ctx context.Context, cmd string) (string, error)
}
