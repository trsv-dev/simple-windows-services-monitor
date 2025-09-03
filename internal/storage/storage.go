package storage

// Storage Интерфейс хранилища.
type Storage interface {
	ServerStorage
	ServiceStorage
	UserStorage
	Close() error
}
