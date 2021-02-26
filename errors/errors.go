package errors

import (
	"errors"
	"net/http"
)

var (
	NotFound  = HttpError{http.StatusNotFound, errors.New("not found")}
	Duplicate = HttpError{http.StatusConflict, errors.New("duplicate")}
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
