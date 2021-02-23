package errors

import "errors"

var (
	NotFound = errors.New("not found")
	Duplicate = errors.New("duplicate")
)
