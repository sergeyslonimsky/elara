package storage

import "errors"

var (
	ErrResourceAlreadyExists = errors.New("resource already exists")
	ErrResourceNotFound      = errors.New("resource not found")
)
