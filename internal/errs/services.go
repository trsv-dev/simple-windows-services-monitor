package errs

import "fmt"

// ErrDuplicatedService Кастомная ошибка, сообщающая о том, служба уже была добавлена пользователем на сервер.
type ErrDuplicatedService struct {
	ServiceName string
	Err         error
}

func (ds *ErrDuplicatedService) Error() string {
	return fmt.Sprintf("Служба %s уже была добавлена на сервер. Ошибка: %v", ds.ServiceName, ds.Err)
}

func (ds *ErrDuplicatedService) Unwrap() error {
	return ds.Err
}

func NewErrDuplicatedService(ServiceName string, err error) *ErrDuplicatedService {
	return &ErrDuplicatedService{
		ServiceName: ServiceName,
		Err:         err,
	}
}

// ErrServiceNotFound Кастомная ошибка, сообщающая о том, что служба не найдена (была удалена или не принадлежит серверу).
type ErrServiceNotFound struct {
	Err       error
	UserID    int64
	ServerID  int64
	ServiceID int64
}

func (no *ErrServiceNotFound) Error() string {
	return fmt.Sprintf("Служба id=%d не найдена среди служб сервера id=%d. Пользователь - id=`%d`. Ошибка: %s", no.ServiceID, no.ServerID, no.UserID, no.Err)
}

func (no *ErrServiceNotFound) Unwrap() error {
	return no.Err
}

func NewErrServiceNotFound(userID int64, serverID int64, serviceID int64, err error) *ErrServiceNotFound {
	if err == nil {
		err = fmt.Errorf("служба не найдена")
	}

	return &ErrServiceNotFound{
		Err:       err,
		UserID:    userID,
		ServerID:  serverID,
		ServiceID: serviceID,
	}
}

// ServiceError Кастомная ошибка, сообщающая о том, что команда для управления состоянием службы завершилась с ошибкой.
type ServiceError struct {
	Code    int
	Message string
}

func NewServiceError(message string, code int) *ServiceError {
	return &ServiceError{Code: code, Message: message}
}

func (se *ServiceError) Error() string {
	return fmt.Sprintf("Код %d, %s", se.Code, se.Message)
}
