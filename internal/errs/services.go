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
	Login     string
	ServerID  int
	ServiceID int
}

func (no *ErrServiceNotFound) Error() string {
	return fmt.Sprintf("Служба id=%d не найдена среди служб сервера id=%d. Пользователь - `%s`. Ошибка: %s", no.ServiceID, no.ServerID, no.Login, no.Err)
}

func (no *ErrServiceNotFound) Unwrap() error {
	return no.Err
}

func NewErrServiceNotFound(login string, serverID int, serviceID int, err error) *ErrServiceNotFound {
	if err == nil {
		err = fmt.Errorf("служба не найдена")
	}

	return &ErrServiceNotFound{
		Err:       err,
		Login:     login,
		ServerID:  serverID,
		ServiceID: serviceID,
	}
}
