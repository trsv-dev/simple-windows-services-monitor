package health_storage

import (
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	statusCacheStorageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/health_storage/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
)

// createTestServerStatus Создает тестовый ServerStatus с заданными параметрами.
func createTestServerStatus(serverID int64, address, status string) models.ServerStatus {
	return models.ServerStatus{
		ServerID: serverID,
		Address:  address,
		Status:   status,
	}
}

// ============================================================================
// БАЗОВЫЕ ЮНИТ-ТЕСТЫ
// ============================================================================

// TestNewStatusCache Проверяет создание нового экземпляра StatusCache.
func TestNewStatusCache(t *testing.T) {
	// создаем новый кэш
	cache := NewStatusCache()

	// проверяем, что кэш и его поля инициализированы корректно
	assert.NotNil(t, cache, "statusCache не должен быть nil")
	assert.NotNil(t, cache.mu, "mutex не должен быть nil")
	assert.NotNil(t, cache.cache, "кэш не должен быть nil")
	assert.Empty(t, cache.cache, "кэш должен быть пустым при создании")
}

// TestStatusCacheSet Проверяет сохранение статуса сервера в кэш.
func TestStatusCacheSet(t *testing.T) {
	// подготавливаем кэш и тестовые данные
	cache := NewStatusCache()
	serverStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// добавляем статус в кэш
	cache.Set(serverStatus)

	// проверяем, что элемент был добавлен корректно
	assert.Len(t, cache.cache, 1, "в кэше должен быть один элемент после set")
	assert.Equal(t, serverStatus, cache.cache[1])
	assert.Equal(t, "192.168.1.10", cache.cache[1].Address)
	assert.Equal(t, "OK", cache.cache[1].Status)
}

// TestStatusCacheSetMultiple Проверяет сохранение нескольких элементов.
func TestStatusCacheSetMultiple(t *testing.T) {
	// подготавливаем кэш и несколько статусов
	cache := NewStatusCache()
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(2, "192.168.1.20", "Unreachable")
	status3 := createTestServerStatus(3, "192.168.1.30", "OK")

	// добавляем все статусы в кэш
	cache.Set(status1)
	cache.Set(status2)
	cache.Set(status3)

	// проверяем, что все элементы были добавлены
	assert.Len(t, cache.cache, 3, "в кэше должно быть три элемента")
	assert.Equal(t, status1, cache.cache[1])
	assert.Equal(t, status2, cache.cache[2])
	assert.Equal(t, status3, cache.cache[3])
}

// TestStatusCacheSetOverwrite Проверяет перезапись значения для того же ID.
func TestStatusCacheSetOverwrite(t *testing.T) {
	// подготавливаем кэш со старым и новым статусом
	cache := NewStatusCache()
	oldStatus := createTestServerStatus(1, "192.168.1.10", "OK")
	newStatus := createTestServerStatus(1, "192.168.2.20", "Unreachable")

	// добавляем старый статус, а потом перезаписываем его
	cache.Set(oldStatus)
	cache.Set(newStatus)

	// проверяем, что был перезаписан именно этот элемент
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, "Unreachable", cache.cache[1].Status)
	assert.Equal(t, "192.168.2.20", cache.cache[1].Address)
}

// TestStatusCacheSetDifferentAddressSameID Проверяет обновление адреса для одного ServerID.
func TestStatusCacheSetDifferentAddressSameID(t *testing.T) {
	// подготавливаем кэш с разными адресами для одного id
	cache := NewStatusCache()
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(1, "192.168.1.100", "Unreachable")

	// добавляем оба статуса, второй должен переписать первый
	cache.Set(status1)
	cache.Set(status2)

	// проверяем, что остался только один элемент с новым адресом
	assert.Len(t, cache.cache, 1)
	assert.Equal(t, "192.168.1.100", cache.cache[1].Address)
}

// TestStatusCacheGet Проверяет получение существующего статуса сервера.
func TestStatusCacheGet(t *testing.T) {
	// подготавливаем кэш с тестовым статусом
	cache := NewStatusCache()
	expectedStatus := createTestServerStatus(42, "10.0.0.1", "OK")
	cache.Set(expectedStatus)

	// получаем статус по id
	status, found := cache.Get(42)

	// проверяем, что получен именно тот статус, который добавили
	assert.True(t, found)
	assert.Equal(t, expectedStatus, status)
	assert.Equal(t, int64(42), status.ServerID)
	assert.Equal(t, "10.0.0.1", status.Address)
	assert.Equal(t, "OK", status.Status)
}

// TestStatusCacheGetNotFound Проверяет получение несуществующего статуса.
func TestStatusCacheGetNotFound(t *testing.T) {
	// подготавливаем пустой кэш
	cache := NewStatusCache()

	// пытаемся получить несуществующий статус
	status, found := cache.Get(999)

	// проверяем, что not found вернулся, и статус нулевой
	assert.False(t, found)
	assert.Equal(t, models.ServerStatus{}, status)
	assert.Equal(t, int64(0), status.ServerID)
	assert.Empty(t, status.Address)
	assert.Empty(t, status.Status)
}

// TestStatusCacheGetFromEmpty Проверяет получение из пустого кэша.
func TestStatusCacheGetFromEmpty(t *testing.T) {
	// подготавливаем новый пустой кэш
	cache := NewStatusCache()

	// пытаемся получить элемент из пустого кэша
	status, found := cache.Get(1)

	// проверяем, что получили empty статус
	assert.False(t, found)
	assert.Equal(t, models.ServerStatus{}, status)
}

// TestStatusCacheDelete Проверяет удаление элемента из кэша.
func TestStatusCacheDelete(t *testing.T) {
	// подготавливаем кэш с одним элементом
	cache := NewStatusCache()
	status := createTestServerStatus(1, "192.168.1.10", "OK")
	cache.Set(status)

	// удаляем элемент из кэша
	cache.Delete(1)

	// проверяем, что элемент удален
	_, found := cache.Get(1)
	assert.False(t, found)
	assert.Empty(t, cache.cache)
}

// TestStatusCacheDeleteMultiple Проверяет удаление одного элемента при наличии нескольких.
func TestStatusCacheDeleteMultiple(t *testing.T) {
	// подготавливаем кэш с несколькими элементами
	cache := NewStatusCache()
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(2, "192.168.1.20", "Unreachable")
	status3 := createTestServerStatus(3, "192.168.1.30", "OK")

	cache.Set(status1)
	cache.Set(status2)
	cache.Set(status3)

	// удаляем средний элемент
	cache.Delete(2)

	// проверяем, что удален именно элемент с id=2
	assert.Len(t, cache.cache, 2)

	_, found := cache.Get(2)
	assert.False(t, found)

	// проверяем, что остальные элементы не тронуты
	status, found := cache.Get(1)
	assert.True(t, found)
	assert.Equal(t, status1, status)

	status, found = cache.Get(3)
	assert.True(t, found)
	assert.Equal(t, status3, status)
}

// TestStatusCacheDeleteNonExistent Проверяет удаление несуществующего элемента.
func TestStatusCacheDeleteNonExistent(t *testing.T) {
	// подготавливаем кэш с одним элементом
	cache := NewStatusCache()
	cache.Set(createTestServerStatus(1, "192.168.1.10", "OK"))

	// удаляем несуществующий элемент (не должно быть паники)
	assert.NotPanics(t, func() {
		cache.Delete(999)
	})

	// проверяем, что имеющийся элемент не пострадал
	_, found := cache.Get(1)
	assert.True(t, found)
}

// TestStatusCacheEmptyStatus Проверяет работу с пустым статусом.
func TestStatusCacheEmptyStatus(t *testing.T) {
	// подготавливаем кэш и статус с пустым status полем
	cache := NewStatusCache()
	status := createTestServerStatus(1, "192.168.1.10", "")

	// добавляем статус в кэш
	cache.Set(status)
	retrieved, found := cache.Get(1)

	// проверяем, что статус с пустым status полем сохранился корректно
	assert.True(t, found)
	assert.Equal(t, "", retrieved.Status)
	assert.Equal(t, "192.168.1.10", retrieved.Address)
}

// ============================================================================
// ТЕСТЫ КОНКУРЕНТНОСТИ
// ============================================================================

// TestStatusCacheConcurrentSet Проверяет конкурентное добавление элементов.
func TestStatusCacheConcurrentSet(t *testing.T) {
	// подготавливаем кэш и запускаем несколько горутин
	cache := NewStatusCache()
	numGoroutines := 100
	var wg sync.WaitGroup

	// добавляем элементы конкурентно из разных горутин
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			status := createTestServerStatus(id, "192.168.1."+string(rune(id+48)), "OK")
			cache.Set(status)
		}(int64(i))
	}

	// ждем, пока все горутины завершат работу
	wg.Wait()

	// проверяем, что все элементы были добавлены корректно
	assert.Len(t, cache.cache, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		status, found := cache.Get(int64(i))
		assert.True(t, found)
		assert.Equal(t, int64(i), status.ServerID)
		assert.Equal(t, "OK", status.Status)
	}
}

// TestStatusCacheConcurrentGet Проверяет конкурентное чтение элементов.
func TestStatusCacheConcurrentGet(t *testing.T) {
	// подготавливаем кэш с одним элементом
	cache := NewStatusCache()
	status := createTestServerStatus(1, "192.168.1.10", "OK")
	cache.Set(status)

	// запускаем несколько горутин для одновременного чтения
	numGoroutines := 100
	var wg sync.WaitGroup
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, found := cache.Get(1)
			results <- found
		}()
	}

	// ждем завершения и закрываем канал
	wg.Wait()
	close(results)

	// проверяем, что все горутины успешно получили элемент
	for result := range results {
		assert.True(t, result)
	}
}

// TestStatusCacheConcurrentDelete Проверяет конкурентное удаление элементов.
func TestStatusCacheConcurrentDelete(t *testing.T) {
	// подготавливаем кэш со множеством элементов
	cache := NewStatusCache()
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		status := createTestServerStatus(int64(i), "192.168.1.10", "OK")
		cache.Set(status)
	}

	var wg sync.WaitGroup

	// удаляем половину элементов конкурентно
	for i := 0; i < numGoroutines/2; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			cache.Delete(id)
		}(int64(i))
	}

	// ждем завершения всех удалений
	wg.Wait()

	// проверяем, что остались только вторая половина элементов
	assert.Len(t, cache.cache, numGoroutines/2)

	for i := 0; i < numGoroutines/2; i++ {
		_, found := cache.Get(int64(i))
		assert.False(t, found)
	}

	for i := numGoroutines / 2; i < numGoroutines; i++ {
		_, found := cache.Get(int64(i))
		assert.True(t, found)
	}
}

// TestStatusCacheConcurrentMixed Проверяет смешанные конкурентные операции Set/Get/Delete.
func TestStatusCacheConcurrentMixed(t *testing.T) {
	// подготавливаем кэш и параметры для смешанных операций
	cache := NewStatusCache()
	numIterations := 50
	var wg sync.WaitGroup

	// конкурентно добавляем элементы
	for i := 0; i < numIterations; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			status := createTestServerStatus(id, "192.168.1.1", "OK")
			cache.Set(status)
		}(int64(i))
	}

	// конкурентно читаем элементы
	for i := 0; i < numIterations; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			cache.Get(id)
		}(int64(i))
	}

	// конкурентно удаляем половину элементов
	for i := 0; i < numIterations/2; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			cache.Delete(id)
		}(int64(i))
	}

	// ждем завершения всех операций
	wg.Wait()

	// проверяем, что в кэше осталось хотя бы половина элементов
	assert.True(t, len(cache.cache) >= numIterations/2)
}

// TestStatusCacheGetSetConcurrent Проверяет конкурентное чтение и запись одного ключа.
func TestStatusCacheGetSetConcurrent(t *testing.T) {
	// подготавливаем кэш с начальным статусом
	cache := NewStatusCache()
	initialStatus := createTestServerStatus(1, "192.168.1.10", "OK")
	cache.Set(initialStatus)

	// запускаем две горутины: одна пишет, другая читает
	done := make(chan bool)
	iterations := 1000

	// горутина, которая записывает изменяющиеся статусы
	go func() {
		for i := 0; i < iterations; i++ {
			status := "OK"
			if i%2 == 0 {
				status = "Unreachable"
			}
			newStatus := createTestServerStatus(1, "192.168.1.10", status)
			cache.Set(newStatus)
		}
		done <- true
	}()

	// горутина, которая читает статусы
	go func() {
		for i := 0; i < iterations; i++ {
			status, found := cache.Get(1)
			assert.True(t, found)
			assert.Equal(t, int64(1), status.ServerID)
			assert.Equal(t, "192.168.1.10", status.Address)
			assert.True(t, status.Status == "OK" || status.Status == "Unreachable")
		}
		done <- true
	}()

	// ждем завершения обеих горутин
	<-done
	<-done

	// проверяем финальное состояние кэша
	finalStatus, found := cache.Get(1)
	assert.True(t, found)
	assert.Equal(t, int64(1), finalStatus.ServerID)
	assert.Equal(t, "192.168.1.10", finalStatus.Address)
}

// ============================================================================
// ИНТЕГРАЦИОННЫЕ ТЕСТЫ
// ============================================================================

// TestStatusCacheIntegrationScenario Проверяет полный сценарий использования кэша.
func TestStatusCacheIntegrationScenario(t *testing.T) {
	// подготавливаем кэш и список серверов
	cache := NewStatusCache()

	servers := []models.ServerStatus{
		createTestServerStatus(1, "192.168.1.10", "OK"),
		createTestServerStatus(2, "192.168.1.20", "OK"),
		createTestServerStatus(3, "192.168.1.30", "Unreachable"),
	}

	// добавляем все серверы в кэш
	for _, server := range servers {
		cache.Set(server)
	}

	// проверяем, что все серверы добавлены
	assert.Len(t, cache.cache, 3)

	// изменяем статус второго сервера
	updatedServer := createTestServerStatus(2, "192.168.1.20", "Unreachable")
	cache.Set(updatedServer)

	// удаляем третий сервер
	cache.Delete(3)

	// проверяем финальное состояние: должны остаться 2 сервера
	assert.Len(t, cache.cache, 2)

	// проверяем, что первый сервер не изменился
	status1, found1 := cache.Get(1)
	require.True(t, found1)
	assert.Equal(t, servers[0], status1)
	assert.Equal(t, "192.168.1.10", status1.Address)
	assert.Equal(t, "OK", status1.Status)

	// проверяем, что второй сервер изменился
	status2, found2 := cache.Get(2)
	require.True(t, found2)
	assert.Equal(t, updatedServer, status2)
	assert.Equal(t, "192.168.1.20", status2.Address)
	assert.Equal(t, "Unreachable", status2.Status)

	// проверяем, что третий сервер удален
	_, found3 := cache.Get(3)
	assert.False(t, found3)
}

// ============================================================================
// ТЕСТЫ С ИСПОЛЬЗОВАНИЕМ МОКА (MockStatusCacheStorage)
// ============================================================================

// TestMockSet Проверяет вызов метода Set у мока.
func TestMockSet(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	testStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание: Set должен быть вызван 1 раз с точным аргументом
	mockStorage.EXPECT().
		Set(gomock.Eq(testStatus)).
		Times(1)

	// выполняем вызов
	mockStorage.Set(testStatus)
	// проверка выполняется автоматически при завершении теста
}

// TestMockSetMultipleTimes Проверяет несколько последовательных вызовов Set у мока.
func TestMockSetMultipleTimes(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(2, "192.168.1.20", "Unreachable")

	// устанавливаем ожидание строгого порядка вызовов
	gomock.InOrder(
		mockStorage.EXPECT().Set(gomock.Eq(status1)).Times(1),
		mockStorage.EXPECT().Set(gomock.Eq(status2)).Times(1),
	)

	// вызываем в установленном порядке
	mockStorage.Set(status1)
	mockStorage.Set(status2)
}

// TestMockSetAnyServerStatus Проверяет вызовы Set с любым ServerStatus.
func TestMockSetAnyServerStatus(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// устанавливаем ожидание: Set может быть вызван с любым аргументом 3 раза
	mockStorage.EXPECT().
		Set(gomock.Any()).
		Times(3)

	// вызываем Set 3 раза с разными аргументами
	mockStorage.Set(createTestServerStatus(1, "192.168.1.10", "OK"))
	mockStorage.Set(createTestServerStatus(2, "192.168.1.20", "Unreachable"))
	mockStorage.Set(createTestServerStatus(3, "192.168.1.30", "OK"))
}

// TestMockGet Проверяет вызов метода Get у мока.
func TestMockGet(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	expectedStatus := createTestServerStatus(42, "10.0.0.1", "OK")

	// устанавливаем ожидание: Get с id=42 должен вернуть expectedStatus и true
	mockStorage.EXPECT().
		Get(gomock.Eq(int64(42))).
		Return(expectedStatus, true).
		Times(1)

	// вызываем Get
	status, found := mockStorage.Get(42)

	// проверяем результаты
	assert.True(t, found)
	assert.Equal(t, expectedStatus, status)
	assert.Equal(t, int64(42), status.ServerID)
	assert.Equal(t, "10.0.0.1", status.Address)
}

// TestMockGetNotFound Проверяет получение несуществующего статуса у мока.
func TestMockGetNotFound(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// устанавливаем ожидание: Get с id=999 должен вернуть empty status и false
	mockStorage.EXPECT().
		Get(gomock.Eq(int64(999))).
		Return(models.ServerStatus{}, false).
		Times(1)

	// вызываем Get
	status, found := mockStorage.Get(999)

	// проверяем, что получили not found
	assert.False(t, found)
	assert.Equal(t, models.ServerStatus{}, status)
}

// TestMockGetAnyID Проверяет вызовы Get с любым ID у мока.
func TestMockGetAnyID(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	expectedStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание: Get может быть вызван с любым id 3 раза
	mockStorage.EXPECT().
		Get(gomock.Any()).
		Return(expectedStatus, true).
		Times(3)

	// вызываем Get 3 раза с разными id
	status1, found1 := mockStorage.Get(1)
	status2, found2 := mockStorage.Get(100)
	status3, found3 := mockStorage.Get(999)

	// проверяем, что все вызовы вернули expectedStatus
	assert.True(t, found1)
	assert.True(t, found2)
	assert.True(t, found3)
	assert.Equal(t, expectedStatus, status1)
	assert.Equal(t, expectedStatus, status2)
	assert.Equal(t, expectedStatus, status3)
}

// TestMockDelete Проверяет вызов метода Delete у мока.
func TestMockDelete(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// устанавливаем ожидание: Delete должен быть вызван с id=1 один раз
	mockStorage.EXPECT().
		Delete(gomock.Eq(int64(1))).
		Times(1)

	// вызываем Delete
	mockStorage.Delete(1)
}

// TestMockDeleteMultiple Проверяет несколько последовательных вызовов Delete у мока.
func TestMockDeleteMultiple(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// устанавливаем ожидание строгого порядка вызовов Delete
	gomock.InOrder(
		mockStorage.EXPECT().Delete(gomock.Eq(int64(1))).Times(1),
		mockStorage.EXPECT().Delete(gomock.Eq(int64(2))).Times(1),
		mockStorage.EXPECT().Delete(gomock.Eq(int64(3))).Times(1),
	)

	// вызываем в установленном порядке
	mockStorage.Delete(1)
	mockStorage.Delete(2)
	mockStorage.Delete(3)
}

// TestMockDeleteAnyID Проверяет вызовы Delete с любым ID у мока.
func TestMockDeleteAnyID(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// устанавливаем ожидание: Delete может быть вызван с любым id 3 раза
	mockStorage.EXPECT().
		Delete(gomock.Any()).
		Times(3)

	// вызываем Delete 3 раза с разными id
	mockStorage.Delete(1)
	mockStorage.Delete(2)
	mockStorage.Delete(3)
}

// TestMockSetAndGet Проверяет последовательность вызовов Set -> Get у мока.
func TestMockSetAndGet(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	testStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание: Set должен быть вызван перед Get
	gomock.InOrder(
		mockStorage.EXPECT().Set(gomock.Eq(testStatus)).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(1))).Return(testStatus, true).Times(1),
	)

	// вызываем в установленном порядке
	mockStorage.Set(testStatus)
	status, found := mockStorage.Get(1)

	// проверяем результаты
	assert.True(t, found)
	assert.Equal(t, testStatus, status)
}

// TestMockWorkflow Проверяет полный workflow: Set -> Get -> Delete -> Get у мока.
func TestMockWorkflow(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	testStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание строгой последовательности операций
	gomock.InOrder(
		mockStorage.EXPECT().Set(gomock.Eq(testStatus)).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(1))).Return(testStatus, true).Times(1),
		mockStorage.EXPECT().Delete(gomock.Eq(int64(1))).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(1))).Return(models.ServerStatus{}, false).Times(1),
	)

	// выполняем workflow
	mockStorage.Set(testStatus)
	status1, found1 := mockStorage.Get(1)
	mockStorage.Delete(1)
	status2, found2 := mockStorage.Get(1)

	// проверяем результаты
	assert.True(t, found1)
	assert.Equal(t, testStatus, status1)
	assert.False(t, found2)
	assert.Equal(t, models.ServerStatus{}, status2)
}

// TestMockMultipleInstances Проверяет работу с несколькими моками одновременно.
func TestMockMultipleInstances(t *testing.T) {
	// создаем мок контроллер и два отдельных мока
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage1 := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	mockStorage2 := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)

	// подготавливаем тестовые данные для каждого мока
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(2, "192.168.1.20", "Unreachable")

	// устанавливаем ожидания для каждого мока независимо
	mockStorage1.EXPECT().Set(gomock.Eq(status1)).Times(1)
	mockStorage2.EXPECT().Set(gomock.Eq(status2)).Times(1)

	// вызываем методы на каждом моке
	mockStorage1.Set(status1)
	mockStorage2.Set(status2)
}

// TestMockComplexWorkflow Проверяет сложный workflow с несколькими операциями у мока.
func TestMockComplexWorkflow(t *testing.T) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	// подготавливаем три разных статуса
	status1 := createTestServerStatus(1, "192.168.1.10", "OK")
	status2 := createTestServerStatus(2, "192.168.1.20", "Unreachable")
	status3 := createTestServerStatus(3, "192.168.1.30", "OK")

	// устанавливаем ожидание сложной последовательности операций
	gomock.InOrder(
		mockStorage.EXPECT().Set(gomock.Eq(status1)).Times(1),
		mockStorage.EXPECT().Set(gomock.Eq(status2)).Times(1),
		mockStorage.EXPECT().Set(gomock.Eq(status3)).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(1))).Return(status1, true).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(2))).Return(status2, true).Times(1),
		mockStorage.EXPECT().Delete(gomock.Eq(int64(2))).Times(1),
		mockStorage.EXPECT().Get(gomock.Eq(int64(2))).Return(models.ServerStatus{}, false).Times(1),
	)

	// выполняем сложный workflow
	mockStorage.Set(status1)
	mockStorage.Set(status2)
	mockStorage.Set(status3)
	st1, f1 := mockStorage.Get(1)
	st2, f2 := mockStorage.Get(2)
	mockStorage.Delete(2)
	st2After, f2After := mockStorage.Get(2)

	// проверяем результаты всех операций
	assert.True(t, f1)
	assert.Equal(t, status1, st1)
	assert.True(t, f2)
	assert.Equal(t, status2, st2)
	assert.False(t, f2After)
	assert.Equal(t, models.ServerStatus{}, st2After)
}

// ============================================================================
// БЕНЧМАРКИ
// ============================================================================

// BenchmarkStatusCacheSet Измеряет производительность операции Set.
func BenchmarkStatusCacheSet(b *testing.B) {
	// подготавливаем кэш и тестовый статус
	cache := NewStatusCache()
	status := createTestServerStatus(1, "192.168.1.10", "OK")

	// сбрасываем таймер и проводим бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(status)
	}
}

// BenchmarkStatusCacheGet Измеряет производительность операции Get.
func BenchmarkStatusCacheGet(b *testing.B) {
	// подготавливаем кэш с тестовым статусом
	cache := NewStatusCache()
	cache.Set(createTestServerStatus(1, "192.168.1.10", "OK"))

	// сбрасываем таймер и проводим бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(1)
	}
}

// BenchmarkStatusCacheDelete Измеряет производительность операции Delete.
func BenchmarkStatusCacheDelete(b *testing.B) {
	// подготавливаем кэш и тестовый статус
	cache := NewStatusCache()
	status := createTestServerStatus(1, "192.168.1.10", "OK")

	// сбрасываем таймер и проводим бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Delete(1)
		cache.Set(status)
	}
}

// BenchmarkStatusCacheConcurrent Измеряет производительность конкурентных операций.
func BenchmarkStatusCacheConcurrent(b *testing.B) {
	// подготавливаем кэш и переменную для синхронизации
	cache := NewStatusCache()
	var wg sync.WaitGroup

	// сбрасываем таймер и проводим конкурентный бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			status := createTestServerStatus(id, "192.168.1.10", "OK")
			// выполняем все три операции в одной горутине
			cache.Set(status)
			cache.Get(id)
			cache.Delete(id)
		}(int64(i % 100))
	}
	// ждем завершения всех горутин
	wg.Wait()
}

// BenchmarkStatusCacheSetWithManyItems Измеряет операцию Set при большом количестве элементов.
func BenchmarkStatusCacheSetWithManyItems(b *testing.B) {
	// подготавливаем кэш с 1000 элементов
	cache := NewStatusCache()

	for i := 0; i < 1000; i++ {
		status := createTestServerStatus(int64(i), "192.168.1.1", "OK")
		cache.Set(status)
	}

	// сбрасываем таймер и проводим бенчмарк добавления нового элемента
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := createTestServerStatus(1001, "192.168.1.10", "OK")
		cache.Set(status)
	}
}

// BenchmarkMockSet Измеряет производительность мока Set.
func BenchmarkMockSet(b *testing.B) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	status := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание: Set будет вызван b.N раз
	mockStorage.EXPECT().
		Set(gomock.Eq(status)).
		Times(b.N)

	// сбрасываем таймер и проводим бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockStorage.Set(status)
	}
}

// BenchmarkMockGet Измеряет производительность мока Get.
func BenchmarkMockGet(b *testing.B) {
	// создаем мок контроллер и мок хранилище
	ctrl := gomock.NewController(b)
	defer ctrl.Finish()

	mockStorage := statusCacheStorageMocks.NewMockStatusCacheStorage(ctrl)
	expectedStatus := createTestServerStatus(1, "192.168.1.10", "OK")

	// устанавливаем ожидание: Get будет вызван b.N раз
	mockStorage.EXPECT().
		Get(gomock.Eq(int64(1))).
		Return(expectedStatus, true).
		Times(b.N)

	// сбрасываем таймер и проводим бенчмарк
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockStorage.Get(1)
	}
}
