package storage

// Storage Интерфейс хранилища.
type Storage interface {
	ServerStorage
	ServiceStorage
	Close() error
}
