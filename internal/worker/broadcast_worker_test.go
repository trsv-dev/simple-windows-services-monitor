package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"

	broadcastMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/broadcast/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	storageMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	// инициализируем логгер для всех тестов
	logger.InitLogger("error", "stdout")
}

// TestFetchAndPublishSuccess Проверяет успешное получение и публикацию статусов.
func TestFetchAndPublishSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	users := []*models.User{
		{ID: 1, Login: "user1"},
		{ID: 2, Login: "user2"},
	}

	statuses1 := []*models.ServiceStatus{
		{ID: 1, ServerID: 1, Status: "running", UpdatedAt: time.Now()},
	}

	statuses2 := []*models.ServiceStatus{
		{ID: 2, ServerID: 2, Status: "stopped", UpdatedAt: time.Now()},
	}

	// ListUsers успешен
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	// GetUserServiceStatuses для первого пользователя
	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(1)).
		Return(statuses1, nil)

	// GetUserServiceStatuses для второго пользователя
	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(2)).
		Return(statuses2, nil)

	// Publish для первого пользователя
	mockBroadcaster.EXPECT().
		Publish("user-1", gomock.Any()).
		Return(nil)

	// Publish для второго пользователя
	mockBroadcaster.EXPECT().
		Publish("user-2", gomock.Any()).
		Return(nil)

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}

// TestFetchAndPublishListUsersError Проверяет ошибку при получении списка пользователей.
func TestFetchAndPublishListUsersError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	// ListUsers возвращает ошибку
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(nil, errors.New("database error"))

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
}

// TestFetchAndPublishGetUserServiceStatusesError Проверяет ошибку при получении статусов пользователя.
func TestFetchAndPublishGetUserServiceStatusesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	users := []*models.User{
		{ID: 1, Login: "user1"},
		{ID: 2, Login: "user2"},
	}

	statuses1 := []*models.ServiceStatus{
		{ID: 1, ServerID: 1, Status: "running", UpdatedAt: time.Now()},
	}

	// ListUsers успешен
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	// GetUserServiceStatuses для первого пользователя успешен
	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(1)).
		Return(statuses1, nil)

	// GetUserServiceStatuses для второго пользователя возвращает ошибку (но она игнорируется с continue)
	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(2)).
		Return(nil, errors.New("user 2 error"))

	// Publish для первого пользователя
	mockBroadcaster.EXPECT().
		Publish("user-1", gomock.Any()).
		Return(nil)

	ctx := context.Background()

	// ошибка GetUserServiceStatuses не должна остановить весь процесс
	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}

// TestFetchAndPublishPublishError Проверяет ошибку при публикации.
func TestFetchAndPublishPublishError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	users := []*models.User{
		{ID: 1, Login: "user1"},
	}

	statuses := []*models.ServiceStatus{
		{ID: 1, ServerID: 1, Status: "running", UpdatedAt: time.Now()},
	}

	// ListUsers успешен
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	// GetUserServiceStatuses успешен
	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(1)).
		Return(statuses, nil)

	// Publish возвращает ошибку
	mockBroadcaster.EXPECT().
		Publish("user-1", gomock.Any()).
		Return(errors.New("publish error"))

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.Error(t, err)
	assert.Equal(t, "publish error", err.Error())
}

// TestFetchAndPublishEmptyUsersList Проверяет пустой список пользователей.
func TestFetchAndPublishEmptyUsersList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	// ListUsers возвращает пустой список
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return([]*models.User{}, nil)

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}

// TestFetchAndPublishContextTimeout Проверяет таймаут контекста.
func TestFetchAndPublishContextTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// даём время на истечение таймаута
	time.Sleep(10 * time.Millisecond)

	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(nil, ctx.Err())

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.Error(t, err)
}

// TestFetchAndPublishTopicFormat Проверяет формат топика для публикации.
func TestFetchAndPublishTopicFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	users := []*models.User{
		{ID: 123, Login: "user123"},
	}

	statuses := []*models.ServiceStatus{
		{ID: 1, ServerID: 1, Status: "running", UpdatedAt: time.Now()},
	}

	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(123)).
		Return(statuses, nil)

	// проверяем что топик имеет правильный формат: "user-{id}"
	mockBroadcaster.EXPECT().
		Publish("user-123", gomock.Any()).
		Return(nil)

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}

// TestFetchAndPublishJsonEncoding Проверяет кодирование JSON.
func TestFetchAndPublishJsonEncoding(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	now := time.Now()
	users := []*models.User{
		{ID: 1, Login: "user1"},
	}

	statuses := []*models.ServiceStatus{
		{ID: 10, ServerID: 5, Status: "running", UpdatedAt: now},
	}

	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(1)).
		Return(statuses, nil)

	// проверяем что данные корректно закодированы
	mockBroadcaster.EXPECT().
		Publish("user-1", gomock.Any()).
		DoAndReturn(func(topic string, data []byte) error {
			var decoded []*models.ServiceStatus
			err := json.Unmarshal(data, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(decoded))
			assert.Equal(t, int64(10), decoded[0].ID)
			assert.Equal(t, int64(5), decoded[0].ServerID)
			assert.Equal(t, "running", decoded[0].Status)
			return nil
		})

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}

// TestBroadcastServiceStatusesContextCancellation Проверяет отмену контекста в воркере.
func TestBroadcastServiceStatusesContextCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	// при первом вызове отменяем контекст
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		DoAndReturn(func(fetchCtx context.Context) ([]*models.User, error) {
			cancel()
			return []*models.User{}, nil
		})

	// запускаем воркер в горутине
	done := make(chan bool)
	go func() {
		BroadcastWorker(ctx, mockStorage, mockBroadcaster, 1*time.Hour)
		done <- true
	}()

	// даём время на завершение
	select {
	case <-done:
		// успешно завершился
	case <-time.After(2 * time.Second):
		t.Fatal("воркер не завершился после отмены контекста")
	}
}

// TestBroadcastServiceStatusesInterval Проверяет периодичность воркера.
func TestBroadcastServiceStatusesInterval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0

	// ожидаем несколько вызовов
	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		DoAndReturn(func(fetchCtx context.Context) ([]*models.User, error) {
			callCount++
			if callCount >= 2 {
				cancel()
			}
			return []*models.User{}, nil
		}).
		AnyTimes()

	start := time.Now()
	BroadcastWorker(ctx, mockStorage, mockBroadcaster, 100*time.Millisecond)
	elapsed := time.Since(start)

	// проверяем что воркер работал по крайней мере 100ms (интервал)
	assert.Greater(t, elapsed, 50*time.Millisecond)
	assert.Greater(t, callCount, 1)
}

// TestBroadcastServiceStatusesMultipleUsers Проверяет работу с несколькими пользователями.
func TestBroadcastServiceStatusesMultipleUsers(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	ctx, cancel := context.WithCancel(context.Background())

	users := []*models.User{
		{ID: 1, Login: "user1"},
		{ID: 2, Login: "user2"},
		{ID: 3, Login: "user3"},
	}

	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		DoAndReturn(func(fetchCtx context.Context) ([]*models.User, error) {
			cancel()
			return users, nil
		})

	// ожидаем публикацию для каждого пользователя
	for i := 1; i <= 3; i++ {
		mockStorage.EXPECT().
			GetUserServiceStatuses(gomock.Any(), int64(i)).
			Return([]*models.ServiceStatus{
				{ID: int64(i), ServerID: int64(i), Status: "running", UpdatedAt: time.Now()},
			}, nil)

		mockBroadcaster.EXPECT().
			Publish(gomock.Any(), gomock.Any()).
			Return(nil)
	}

	BroadcastWorker(ctx, mockStorage, mockBroadcaster, 1*time.Hour)
}

// TestFetchAndPublishMultipleStatusesPerUser Проверяет несколько статусов для одного пользователя.
func TestFetchAndPublishMultipleStatusesPerUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := storageMocks.NewMockStorage(ctrl)
	mockBroadcaster := broadcastMocks.NewMockBroadcaster(ctrl)

	now := time.Now()
	users := []*models.User{
		{ID: 1, Login: "user1"},
	}

	statuses := []*models.ServiceStatus{
		{ID: 1, ServerID: 1, Status: "running", UpdatedAt: now},
		{ID: 2, ServerID: 1, Status: "stopped", UpdatedAt: now},
		{ID: 3, ServerID: 2, Status: "running", UpdatedAt: now},
	}

	mockStorage.EXPECT().
		ListUsers(gomock.Any()).
		Return(users, nil)

	mockStorage.EXPECT().
		GetUserServiceStatuses(gomock.Any(), int64(1)).
		Return(statuses, nil)

	mockBroadcaster.EXPECT().
		Publish("user-1", gomock.Any()).
		DoAndReturn(func(topic string, data []byte) error {
			var decoded []*models.ServiceStatus
			err := json.Unmarshal(data, &decoded)
			assert.NoError(t, err)
			assert.Equal(t, 3, len(decoded))
			return nil
		})

	ctx := context.Background()

	err := fetchAndPublish(ctx, mockStorage, mockBroadcaster)

	assert.NoError(t, err)
}
