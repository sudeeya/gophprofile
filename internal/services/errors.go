package services

import "errors"

var (
	ErrForbidden          = errors.New("forbidden")
	ErrAvatarTooLarge     = errors.New("avatar too large")
	ErrFormatNotSupported = errors.New("format not supported")
)
