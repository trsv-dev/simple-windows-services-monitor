package errs

import "fmt"

// ErrLoginIsTaken Кастомная ошибка, сообщающая, что логин уже был занят.
type ErrLoginIsTaken struct {
	Login string
	Err   error
}

func (lt *ErrLoginIsTaken) Error() string {
	return fmt.Sprintf("Пользователь с логином `%s` уже существует. Ошибка: %v", lt.Login, lt.Err)
}

func (lt *ErrLoginIsTaken) Unwrap() error {
	return lt.Err
}

func NewErrLoginIsTaken(login string, err error) *ErrLoginIsTaken {
	return &ErrLoginIsTaken{
		Login: login,
		Err:   err,
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
	Err error
}

func (nf *ErrUserIDNotFound) Error() string {
	return fmt.Sprintf("Пользователь с таким id не найден. Ошибка: %v", nf.Err)
}

func (nf *ErrUserIDNotFound) Unwrap() error {
	return nf.Err
}

func NewErrUserIDNotFound(err error) *ErrUserIDNotFound {
	return &ErrUserIDNotFound{
		Err: err,
	}
}
