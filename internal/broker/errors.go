package broker

import "errors"

var (
	ErrConnClosed   = errors.New("conn closed")
	ErrNACKReceived = errors.New("nack received")
)
