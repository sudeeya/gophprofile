package storage

import "io"

type PutAvatarInput struct {
	Filename    string
	ContentType string
	Size        int64
	Reader      io.Reader
}

type PutAvatarOutput struct {
	Key string
}

type GetAvatarOutput struct {
	Bytes []byte
}
