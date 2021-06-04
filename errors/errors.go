package errors

import (
	"errors"
	"net/http"
)

var (
	NotFound  = HttpError{http.StatusNotFound, errors.New("not found")}
	Duplicate = HttpError{http.StatusConflict, errors.New("duplicate")}
	ConstraintViolation = HttpError{http.StatusUnprocessableEntity, errors.New("constraint violation")}
)

type HttpError struct {
	Code int
	Err  error
}

func (h HttpError) Unwrap() error {
	return h.Err
}

func (h HttpError) Error() string {
	return h.Err.Error()
}
