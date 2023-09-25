package test

import (
	"fmt"
	"github.com/golang/mock/gomock"
)

type funcMatcher[T any] struct {
	match func(val T) bool
	v     T
}

func (f *funcMatcher[T]) Matches(val interface{}) bool {
	var ok bool
	f.v, ok = val.(T)
	if !ok {
		return false
	}

	return f.match(f.v)
}

func (f *funcMatcher[T]) String() string {
	return fmt.Sprintf("to match %v", f.v)
}

func Match[T any](m func(v T) bool) gomock.Matcher {
	return &funcMatcher[T]{
		match: m,
	}
}
