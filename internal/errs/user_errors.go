package errs

import "fmt"

// ErrUserAlreadyExists Кастомная ошибка, сообщающая, что пользователь уже существует.
type ErrUserAlreadyExists struct {
	ID  string
	Err error
}

func (e *ErrUserAlreadyExists) Error() string {
	return fmt.Sprintf("Пользователь `%s` уже существует. Ошибка: %v", e.ID, e.Err)
}

func (e *ErrUserAlreadyExists) Unwrap() error {
	return e.Err
}

func NewErrUserAlreadyExists(id string, err error) *ErrUserAlreadyExists {
	return &ErrUserAlreadyExists{
		ID:  id,
		Err: err,
	}
}

// ErrWrongLogin Кастомная ошибка, сообщающая о неверном логине.
type ErrWrongLogin struct {
	Err error
}

func (wl *ErrWrongLogin) Error() string {
	return fmt.Sprintf("Неверный логин. Ошибка: %v", wl.Err)
}

func (wl *ErrWrongLogin) Unwrap() error {
	return wl.Err
}

func NewErrWrongLoginOrPassword(err error) *ErrWrongLogin {
	return &ErrWrongLogin{
		Err: err,
	}
}

// ErrUserIDNotFound Кастомная ошибка, сообщающая о том, что пользователь с таким ID не был найден.
type ErrUserIDNotFound struct {
	UserID string
}

func (nf *ErrUserIDNotFound) Error() string {
	return fmt.Sprintf("Пользователь с id = %s не найден", nf.UserID)
}

func NewErrUserIDNotFound(userID string) *ErrUserIDNotFound {
	return &ErrUserIDNotFound{
		UserID: userID,
	}
}
