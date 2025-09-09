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

// ErrWrongLoginOrPassword Кастомная ошибка, сообщающая о неверной паре логин/пароль.
type ErrWrongLoginOrPassword struct {
	Err error
}

func (wl *ErrWrongLoginOrPassword) Error() string {
	return fmt.Sprintf("Неверная пара логин/пароль. Ошибка: %v", wl.Err)
}

func (wl *ErrWrongLoginOrPassword) Unwrap() error {
	return wl.Err
}

func NewErrWrongLoginOrPassword(err error) *ErrWrongLoginOrPassword {
	return &ErrWrongLoginOrPassword{
		Err: err,
	}
}

// ErrLoginNotFound Кастомная ошибка, сообщающая о том, что логин не был найден.
type ErrLoginNotFound struct {
	Err error
}

func (nf *ErrLoginNotFound) Error() string {
	return fmt.Sprintf("Такой логин не найден. Ошибка: %v", nf.Err)
}

func (nf *ErrLoginNotFound) Unwrap() error {
	return nf.Err
}

func NewErrLoginNotFound(err error) *ErrLoginNotFound {
	return &ErrLoginNotFound{
		Err: err,
	}
}
