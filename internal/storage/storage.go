package storage

import "context"

//go:generate mockgen -destination=mocks/storage_mock.go -package=mocks . Storage

// Storage Интерфейс хранилища.
type Storage interface {
	ServerStorage
	ServiceStorage
	UserStorage
	Ping(ctx context.Context) error
	Close() error
}
