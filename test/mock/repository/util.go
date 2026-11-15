package repository_mock

import "github.com/stretchr/testify/mock"

// ret returns args[i] as T, or T's zero value when the mock returned nil.
func ret[T any](args mock.Arguments, i int) T {
	v, _ := args.Get(i).(T)
	return v
}
