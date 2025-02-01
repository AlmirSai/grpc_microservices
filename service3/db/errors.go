package db

import "errors"

var (
	ErrTooManyConnections = errors.New("too many active database connections")
	ErrConnectionTimeout  = errors.New("database connection timeout")
	ErrTransactionFailed  = errors.New("transaction failed")
	ErrInvalidOperation   = errors.New("invalid database operation")
)

type DBError struct {
	Err     error
	Message string
	Code    string
}

func (e *DBError) Error() string {
	return e.Message
}

func (e *DBError) Unwrap() error {
	return e.Err
}

func NewDBError(err error, message string, code string) *DBError {
	return &DBError{
		Err:     err,
		Message: message,
		Code:    code,
	}
}
