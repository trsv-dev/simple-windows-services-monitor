package storage

import "context"

// Storage Интерфейс хранилища.
type Storage interface {
	ServerStorage
	ServiceStorage
	UserStorage
	Ping(ctx context.Context) error
	Close() error
}
