package service

import "errors"

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func NewError(code string, err error) *Error {
	return &Error{Code: code, Err: err}
}

var (
	ErrInvalidAccountPassword = NewError("1001001", errors.New("invalid account password"))
	ErrUpdateAccountBusy      = NewError("1001002", errors.New("update account busy"))
)
