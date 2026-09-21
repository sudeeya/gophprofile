package services

import (
	"bytes"
	"context"
	"testing"
	"uuid"

	"github.com/labstack/echo/v5/echotest"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sudeeya/gophprofile/internal/broker"
	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/storage"
)

func TestUploadAvatar(t *testing.T) {
	var (
		dataJPEG         = echotest.LoadBytes(t, "../testdata/cat.jpeg")
		dataTooLarge     = bytes.Repeat([]byte{'a'}, MaxAvatarSize+1)
		dataNotSupported = []byte("not supported")
	)

	tests := []struct {
		name               string
		data               []byte
		mimeType           string
		id                 uuid.UUID
		key                string
		wantErr            error
		setupRepoMock      func(m *MockAvatarRepository, id uuid.UUID, key, mimeType string, size int)
		setupStorageMock   func(m *MockAvatarStorage, key string)
		setupPublisherMock func(m *MockAvatarEventPublisher, id uuid.UUID, key string)
	}{
		{
			name:     "success",
			data:     dataJPEG,
			mimeType: "image/jpeg",
			id:       uuid.New(),
			key:      "key",
			setupRepoMock: func(m *MockAvatarRepository, id uuid.UUID, key, mimeType string, size int) {
				m.EXPECT().
					CreateAvatar(mock.Anything, repository.CreateAvatarInput{
						MimeType: mimeType,
						S3Key:    key,
						Size:     int64(size),
					}).
					Return(repository.CreateAvatarOutput{ID: id}, nil).
					Once()
			},
			setupStorageMock: func(m *MockAvatarStorage, key string) {
				m.EXPECT().
					PutAvatar(mock.Anything, mock.Anything).
					Return(storage.PutAvatarOutput{Key: key}, nil).
					Once()
			},
			setupPublisherMock: func(m *MockAvatarEventPublisher, id uuid.UUID, key string) {
				m.EXPECT().
					PublishAvatarUploadEvent(mock.Anything, broker.AvatarUploadEvent{ID: id, S3Key: key}).
					Return(nil).
					Once()
			},
		},
		{
			name:    "avatar too large",
			data:    dataTooLarge,
			wantErr: ErrAvatarTooLarge,
		},
		{
			name:    "format not supported",
			data:    dataNotSupported,
			wantErr: ErrFormatNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := NewMockAvatarRepository(t)
			if tt.setupRepoMock != nil {
				tt.setupRepoMock(repo, tt.id, tt.key, tt.mimeType, len(tt.data))
			}

			storage := NewMockAvatarStorage(t)
			if tt.setupStorageMock != nil {
				tt.setupStorageMock(storage, tt.key)
			}

			publisher := NewMockAvatarEventPublisher(t)
			if tt.setupPublisherMock != nil {
				tt.setupPublisherMock(publisher, tt.id, tt.key)
			}

			service := NewAvatarService(repo, storage, publisher)
			avatar, err := service.UploadAvatar(context.Background(), UploadAvatarInput{
				Reader: bytes.NewReader(tt.data),
			})
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.id, avatar.Metadata.ID)
		})
	}
}
