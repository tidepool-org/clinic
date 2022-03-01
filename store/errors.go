package store

import (
	"errors"
	"go.mongodb.org/mongo-driver/mongo"
)

func IsDuplicateKeyError(err error) bool {
	for ; err != nil; err = errors.Unwrap(err) {
		if e, ok := err.(mongo.ServerError); ok {
			return e.HasErrorCode(11000) || e.HasErrorCode(11001) || e.HasErrorCode(12582) ||
				e.HasErrorCodeWithMessage(16460, " E11000 ")
		}
	}
	return false
}
