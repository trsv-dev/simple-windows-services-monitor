package service_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/contextkeys"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	netutilsMocks "github.com/trsv-dev/simple-windows-services-monitor/internal/netutils/mocks"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// TestNewServiceHandler Проверяет создание ServiceHandler.
func TestNewServiceHandler(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	handler := NewServiceHandler(mockStorage, mockChecker)

	// проверяем что handler не nil
	assert.NotNil(t, handler)

	// проверяем что поля установлены
	assert.NotNil(t, handler.storage)
	assert.NotNil(t, handler.checker)
}

// TestAddServiceInvalidJSON Проверяет добавление службы с невалидным JSON.
func TestAddServiceInvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	handler := NewServiceHandler(mockStorage, mockChecker)

	// создаём запрос с невалидным JSON
	body := []byte(`{invalid json}`)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	// добавляем контекст с credentials
	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// проверяем что ответ содержит ошибку
	assert.Contains(t, w.Body.String(), "формат")
}

// TestAddServiceInvalidServiceData Проверяет добавление службы с невалидными данными.
func TestAddServiceInvalidServiceData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)
	handler := NewServiceHandler(mockStorage, mockChecker)

	// создаём service с пустым именем
	service := models.Service{
		ServiceName:   "",
		DisplayedName: "Test",
	}

	body, _ := json.Marshal(service)
	r := httptest.NewRequest(http.MethodPost, "/services", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	r = r.WithContext(ctx)

	handler.AddService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestDelServiceSuccess Проверяет успешное удаление службы.
func TestDelServiceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что DelService будет вызван
	mockStorage.EXPECT().
		DelService(gomock.Any(), int64(1), int64(1), int64(1)).
		Return(nil)

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	// создаём контекст с credentials
	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем что ответ содержит сообщение об успехе
	assert.Contains(t, w.Body.String(), "успешно удалена")
}

// TestDelServiceNotFound Проверяет удаление несуществующей службы.
func TestDelServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что DelService вернёт ошибку
	mockStorage.EXPECT().
		DelService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&errs.ErrServiceNotFound{
			UserID:    1,
			ServerID:  1,
			ServiceID: 1,
			Err:       errors.New("not found"),
		})

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)

	// проверяем сообщение об ошибке
	assert.Contains(t, w.Body.String(), "не найдена")
}

// TestGetServiceSuccess Проверяет успешное получение информации о службе.
func TestGetServiceSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	service := &models.Service{
		ID:            1,
		ServiceName:   "test-service",
		DisplayedName: "Test Service",
		Status:        "running",
	}

	// ожидаем что GetService будет вызван
	mockStorage.EXPECT().
		GetService(gomock.Any(), int64(1), int64(1), int64(1)).
		Return(service, nil)

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем content-type
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем что ответ содержит сервис
	var responseService models.Service
	err := json.NewDecoder(w.Body).Decode(&responseService)
	assert.NoError(t, err)
	assert.Equal(t, "test-service", responseService.ServiceName)
}

// TestGetServiceNotFound Проверяет получение несуществующей службы.
func TestGetServiceNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что GetService вернёт ошибку
	mockStorage.EXPECT().
		GetService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &errs.ErrServiceNotFound{
			UserID:    1,
			ServerID:  1,
			ServiceID: 1,
			Err:       errors.New("not found"),
		})

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)

	// проверяем сообщение об ошибке
	assert.Contains(t, w.Body.String(), "не найдена")
}

// TestGetServicesListSuccess Проверяет успешное получение списка служб.
func TestGetServicesListSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
		{ID: 2, ServiceName: "service2", DisplayedName: "Service 2", Status: "stopped"},
	}

	// ожидаем что ListServices будет вызван
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем content-type
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	// проверяем что ответ содержит два сервиса
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseServices))
}

// TestGetServicesListEmpty Проверяет получение пустого списка служб.
func TestGetServicesListEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что ListServices вернёт пустой слайс
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return([]*models.Service{}, nil)

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusOK, w.Code)

	// проверяем что ответ содержит пустой слайс
	var responseServices []*models.Service
	err := json.NewDecoder(w.Body).Decode(&responseServices)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(responseServices))
}

// TestIsServiceExistsWithStateMarker Проверяет существование службы по маркеру STATE.
func TestIsServiceExistsWithStateMarker(t *testing.T) {
	result := `SERVICE_NAME: TestService
	DISPLAY_NAME: Test Service
	TYPE: 10 WIN32_OWN_PROCESS
	STATE: 4 RUNNING
	WIN32_EXIT_CODE: 0
	SERVICE_EXIT_CODE: 0
	CHECKPOINT: 0
	WAIT_HINT: 0`

	// проверяем что функция вернёт true
	assert.True(t, isServiceExists(result))
}

// TestIsServiceExistsWithServiceNameMarker Проверяет существование службы по маркеру SERVICE_NAME.
func TestIsServiceExistsWithServiceNameMarker(t *testing.T) {
	result := "SERVICE_NAME: someservice"

	// проверяем что функция вернёт true
	assert.True(t, isServiceExists(result))
}

// TestIsServiceExistsWithErrorCode1060 Проверяет отсутствие службы по коду 1060.
func TestIsServiceExistsWithErrorCode1060(t *testing.T) {
	result := `QueryServiceConfig FAILED
	1060`

	// проверяем что функция вернёт false
	assert.False(t, isServiceExists(result))
}

// TestIsServiceExistsWithEmptyResult Проверяет отсутствие службы при пустом результате.
func TestIsServiceExistsWithEmptyResult(t *testing.T) {
	result := ""

	// проверяем что функция вернёт false
	assert.False(t, isServiceExists(result))
}

// TestIsServiceExistsWithUnknownResult Проверяет отсутствие службы при неизвестном результате.
func TestIsServiceExistsWithUnknownResult(t *testing.T) {
	result := "Some random text without markers"

	// проверяем что функция вернёт false
	assert.False(t, isServiceExists(result))
}

// TestGetServicesListWithActualParameter Проверяет получение списка с параметром actual=true.
func TestGetServicesListWithActualParameter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	services := []*models.Service{
		{ID: 1, ServiceName: "service1", DisplayedName: "Service 1", Status: "running"},
	}

	// ожидаем вызовы для получения списка служб
	mockStorage.EXPECT().
		ListServices(gomock.Any(), int64(1), int64(1)).
		Return(services, nil)

	// ожидаем ошибку при получении сервера (так тест будет проще)
	mockStorage.EXPECT().
		GetServerWithPassword(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &errs.ErrServerNotFound{
			UserID:   1,
			ServerID: 1,
			Err:      errors.New("not found"),
		})

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services?actual=true", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDelServiceDatabaseError Проверяет удаление при ошибке БД.
func TestDelServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что DelService вернёт обычную ошибку
	mockStorage.EXPECT().
		DelService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("database error"))

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodDelete, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.DelService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServiceDatabaseError Проверяет получение при ошибке БД.
func TestGetServiceDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что GetService вернёт обычную ошибку
	mockStorage.EXPECT().
		GetService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database error"))

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services/1", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServiceID, int64(1))

	r = r.WithContext(ctx)

	handler.GetService(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServicesListDatabaseError Проверяет получение списка при ошибке БД.
func TestGetServicesListDatabaseError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что ListServices вернёт обычную ошибку
	mockStorage.EXPECT().
		ListServices(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("database error"))

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestGetServicesListServerNotFound Проверяет получение списка когда сервер не найден.
func TestGetServicesListServerNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockChecker := netutilsMocks.NewMockChecker(ctrl)

	// ожидаем что ListServices вернёт ошибку что сервер не найден
	mockStorage.EXPECT().
		ListServices(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &errs.ErrServiceNotFound{
			ServerID: 1,
			Err:      errors.New("server not found"),
		})

	handler := NewServiceHandler(mockStorage, mockChecker)

	r := httptest.NewRequest(http.MethodGet, "/services", nil)
	w := httptest.NewRecorder()

	ctx := context.WithValue(r.Context(), contextkeys.Login, "testuser")
	ctx = context.WithValue(ctx, contextkeys.ID, int64(1))
	ctx = context.WithValue(ctx, contextkeys.ServerID, int64(1))

	r = r.WithContext(ctx)

	handler.GetServicesList(w, r)

	// проверяем статус
	assert.Equal(t, http.StatusNotFound, w.Code)
}
