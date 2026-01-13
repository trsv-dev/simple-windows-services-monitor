package health_storage

import (
	"sync"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// StatusCache Структура для хранения статуса сервера.
type StatusCache struct {
	mu    sync.RWMutex
	cache map[int64]models.ServerStatus
}

// NewStatusCache Конструктор StatusCache.
func NewStatusCache() *StatusCache {
	cache := make(map[int64]models.ServerStatus)

	return &StatusCache{
		cache: cache,
	}
}

// Set Метод для сохранения статуса сервера в in-memory хранилище.
func (sc *StatusCache) Set(s models.ServerStatus) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	old, ok := sc.cache[s.ServerID]
	if ok && old.Status == s.Status {
		return
	}

	sc.cache[s.ServerID] = s
}

// Get Метод для извлечения статуса сервера из in-memory хранилище.
func (sc *StatusCache) Get(id int64) (models.ServerStatus, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	v, ok := sc.cache[id]

	return v, ok
}

// Delete Метод для удаления статуса сервера из in-memory хранилища.
func (sc *StatusCache) Delete(id int64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.cache, id)
}

// GetAllServerStatusesByUser Получение всех статусов серверов пользователя.
func (sc *StatusCache) GetAllServerStatusesByUser(userID int64) []models.ServerStatus {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	var res []models.ServerStatus

	for _, s := range sc.cache {
		if s.UserID == userID {
			res = append(res, s)
		}
	}

	return res
}
