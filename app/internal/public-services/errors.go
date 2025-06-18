package public_services

import "errors"

var (
	ErrServiceNotFound = errors.New("public service not found")
	ErrParamNotFound   = errors.New("service parameter not found")
	ErrParamExists     = errors.New("service parameter already exists")
)
