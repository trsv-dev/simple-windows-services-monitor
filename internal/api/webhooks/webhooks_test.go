package webhooks

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/logger"

	"github.com/trsv-dev/simple-windows-services-monitor/internal/errs"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/models"
	"github.com/trsv-dev/simple-windows-services-monitor/internal/storage/mocks"
)

func init() {
	logger.InitLogger("error", "stdout")
}

// Test_HandleEvent_UserEvent_RegisterSuccess Проверяет успешную обработку UserEvent REGISTER.
func Test_HandleEvent_UserEvent_RegisterSuccess(t *testing.T) {
	// Подготавливаем тело запроса: Keycloak user event REGISTER
	body := []byte(`{
        "type": "REGISTER",
        "userId": "any-id-user-1",
        "ipAddress": "127.0.0.1",
        "details": {
            "username": "testuser"
        }
    }`)

	// создаём контроллер gomock
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// создаём мок storage.Storage
	mockStorage := mocks.NewMockStorage(ctrl)

	// ожидаем, что при регистрации будет вызван CreateUser с нужными полями
	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), &models.User{
			ID:    "any-id-user-1",
			Login: "testuser",
		}).
		Return(nil)

	// Создаём экземпляр Webhook с замоканным storage
	wh := NewWebhook(mockStorage)

	// Собираем HTTP‑запрос
	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	// ResponseRecorder - "фейковый" http.ResponseWriter для тестов
	rr := httptest.NewRecorder()

	// вызываем хендлер верхнего уровня
	wh.HandleEvent(rr, req)

	// проверяем, что код ответа 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_UserEvent_Register_NoUsername Проверяет случай, когда в details нет username.
func Test_HandleEvent_UserEvent_Register_NoUsername(t *testing.T) {
	body := []byte(`{
        "type": "REGISTER",
        "userId": "any-id-user-1",
        "ipAddress": "127.0.0.1",
        "details": {
            "email": "test@example.com"
        }
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	// в этом кейсе CreateUser вызываться не должен
	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		Times(0)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// ожидаем 400 Bad Request из-за отсутствия username
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_UserEvent_Register_UserAlreadyExists Проверяет ErrUserAlreadyExists.
func Test_HandleEvent_UserEvent_Register_UserAlreadyExists(t *testing.T) {
	body := []byte(`{
        "type": "REGISTER",
        "userId": "any-id-user-1",
        "ipAddress": "127.0.0.1",
        "details": {
            "username": "testuser"
        }
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	// Подготавливаем доменную ошибку ErrUserAlreadyExists
	userExistsErr := &errs.ErrUserAlreadyExists{
		ID:  "any-id-user-1",
		Err: errors.New("already exists"),
	}

	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), &models.User{
			ID:    "any-id-user-1",
			Login: "testuser",
		}).
		Return(userExistsErr)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// Ожидаем 409 Conflict
	if rr.Code != http.StatusConflict {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusConflict, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_UserEvent_Register_InternalError Проверяет любую другую ошибку CreateUser.
func Test_HandleEvent_UserEvent_Register_InternalError(t *testing.T) {
	body := []byte(`{
        "type": "REGISTER",
        "userId": "any-id-user-1",
        "ipAddress": "127.0.0.1",
        "details": {
            "username": "testuser"
        }
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		Return(errors.New("db failure"))

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// Ожидаем 500 Internal Server Error
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Delete_Success Проверяет успешное удаление пользователя.
func Test_HandleEvent_AdminEvent_Delete_Success(t *testing.T) {
	// пример тела admin event DELETE USER
	body := []byte(`{
        "operationType": "DELETE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	// ожидаем вызов DeleteUser с userID, который вернет event.ExtractUserID().
	mockStorage.
		EXPECT().
		DeleteUser(gomock.Any(), "any-id-user-1").
		Return(nil)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// при успехе - 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Delete_UserNotFound Проверяет ErrUserIDNotFound.
func Test_HandleEvent_AdminEvent_Delete_UserNotFound(t *testing.T) {
	body := []byte(`{
        "operationType": "DELETE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	// готовим доменную ошибку ErrUserIDNotFound
	notFoundErr := &errs.ErrUserIDNotFound{
		UserID: "any-id-user-1",
	}

	mockStorage.
		EXPECT().
		DeleteUser(gomock.Any(), "any-id-user-1").
		Return(notFoundErr)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// при уже удалённом пользователе - 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Delete_InternalError Проверяет любую другую ошибку DeleteUser.
func Test_HandleEvent_AdminEvent_Delete_InternalError(t *testing.T) {
	body := []byte(`{
        "operationType": "DELETE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	mockStorage.
		EXPECT().
		DeleteUser(gomock.Any(), "any-id-user-1").
		Return(errors.New("db failure"))

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// ожидаем 500 Internal Server Error
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Create_Success Проверяет создание пользователя через admin event CREATE USER.
func Test_HandleEvent_AdminEvent_Create_Success(t *testing.T) {
	// representation - строка с JSON внутри JSON (как в Keycloak admin event)
	body := []byte(`{
        "operationType": "CREATE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1",
        "representation": "{\"id\":\"any-id-user-1\",\"username\":\"testuser\"}"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), &models.User{
			ID:    "any-id-user-1",
			Login: "testuser",
		}).
		Return(nil)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Create_UserAlreadyExists Проверяет ErrUserAlreadyExists при CREATE USER.
func Test_HandleEvent_AdminEvent_Create_UserAlreadyExists(t *testing.T) {
	body := []byte(`{
        "operationType": "CREATE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1",
        "representation": "{\"id\":\"any-id-user-1\",\"username\":\"testuser\"}"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	userExistsErr := &errs.ErrUserAlreadyExists{
		ID:  "any-id-user-1",
		Err: errors.New("already exists"),
	}

	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), &models.User{
			ID:    "any-id-user-1",
			Login: "testuser",
		}).
		Return(userExistsErr)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// ожидаем 409 Conflict
	if rr.Code != http.StatusConflict {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusConflict, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_AdminEvent_Create_InternalError Проверяет любую другую ошибку CreateUser.
func Test_HandleEvent_AdminEvent_Create_InternalError(t *testing.T) {
	body := []byte(`{
        "operationType": "CREATE",
        "resourceType": "USER",
        "resourcePath": "users/any-id-user-1",
        "representation": "{\"id\":\"any-id-user-1\",\"username\":\"testuser\"}"
    }`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)

	mockStorage.
		EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		Return(errors.New("db failure"))

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/keycloak-events", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusInternalServerError, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_InvalidJSON Проверяет неверный JSON в теле запроса.
func Test_HandleEvent_InvalidJSON(t *testing.T) {
	body := []byte(`{invalid json`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	// никакие методы хранилища вызываться не должны
	mockStorage.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(0)
	mockStorage.EXPECT().DeleteUser(gomock.Any(), gomock.Any()).Times(0)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

// Test_HandleEvent_UnknownEventType Проверяет случай, когда нет ни type, ни operationType.
func Test_HandleEvent_UnknownEventType(t *testing.T) {
	body := []byte(`{"foo": "bar"}`)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStorage := mocks.NewMockStorage(ctrl)
	mockStorage.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(0)
	mockStorage.EXPECT().DeleteUser(gomock.Any(), gomock.Any()).Times(0)

	wh := NewWebhook(mockStorage)

	req := httptest.NewRequest(http.MethodPost, "/webhooks", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	wh.HandleEvent(rr, req)

	// в этом случае хендлер просто возвращает 204 No Content
	if rr.Code != http.StatusNoContent {
		t.Fatalf("ожидался статус %d, получен %d, тело ответа: %s", http.StatusNoContent, rr.Code, rr.Body.String())
	}
}
