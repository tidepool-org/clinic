package errors

import (
	"errors"
	"net/http"
)

var (
	NotFound            = HttpError{http.StatusNotFound, errors.New("not found")}
	Duplicate           = HttpError{http.StatusConflict, errors.New("duplicate")}
	ConstraintViolation = HttpError{http.StatusUnprocessableEntity, errors.New("constraint violation")}
	BadRequest          = HttpError{http.StatusBadRequest, errors.New("bad request")}
	Unauthorized        = HttpError{http.StatusUnauthorized, errors.New("unauthorized")}
	PaymentRequired     = HttpError{http.StatusPaymentRequired, errors.New("payment required")}
	InternalServerError = HttpError{http.StatusInternalServerError, errors.New("internal server error")}
	Conflict            = HttpError{http.StatusConflict, errors.New("conflict")}
	NoChange            = HttpError{http.StatusNoContent, errors.New("no change")}
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
