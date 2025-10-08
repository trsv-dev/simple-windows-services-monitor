package errs

import "fmt"

// ErrDuplicatedServer Кастомная ошибка, сообщающая о том, сервер уже был добавлен пользователем.
type ErrDuplicatedServer struct {
	Address string
	Err     error
}

func (ds *ErrDuplicatedServer) Error() string {
	return fmt.Sprintf("Сервер %s уже был добавлен. Ошибка: %v", ds.Address, ds.Err)
}

func (ds *ErrDuplicatedServer) Unwrap() error {
	return ds.Err
}

func NewErrDuplicatedServer(serverAddr string, err error) *ErrDuplicatedServer {
	return &ErrDuplicatedServer{
		Address: serverAddr,
		Err:     err,
	}
}

// ErrServerNotFound Кастомная ошибка, сообщающая о том, что сервер не найден (был удален или не принадлежит пользователю).
type ErrServerNotFound struct {
	Err      error
	ServerID int64
	UserID   int64
}

func (no *ErrServerNotFound) Error() string {
	return fmt.Sprintf("Сервер id=%d не найден среди серверов пользователя id=%d. Ошибка: %s", no.ServerID, no.UserID, no.Err)
}

func (no *ErrServerNotFound) Unwrap() error {
	return no.Err
}

func NewErrServerNotFound(serverID int64, userID int64, err error) *ErrServerNotFound {
	if err == nil {
		err = fmt.Errorf("сервер не найден")
	}

	return &ErrServerNotFound{
		Err:      err,
		ServerID: serverID,
		UserID:   userID,
	}
}
