package services

import (
	"bytes"
	"context"
	"errors"
	"image"
	"testing"
	"uuid"

	"github.com/labstack/echo/v5/echotest"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sudeeya/gophprofile/internal/repository"
	"github.com/sudeeya/gophprofile/internal/storage"
)

func TestGenerateThumbnail(t *testing.T) {
	dataJPEG := echotest.LoadBytes(t, "../testdata/cat.jpeg")

	tests := []struct {
		name             string
		id               uuid.UUID
		key              string
		setupRepoMock    func(m *MockAvatarRepository, id uuid.UUID, key string)
		setupStorageMock func(m *MockAvatarStorage, key string)
	}{
		{
			name: "success",
			id:   uuid.New(),
			key:  "key",
			setupRepoMock: func(m *MockAvatarRepository, id uuid.UUID, key string) {
				m.EXPECT().
					GetAvatar(mock.Anything, id).
					Return(repository.GetAvatarOutput{S3Key: key}, nil).
					Once()
				m.EXPECT().
					GetAvatarMetadata(mock.Anything, id).
					Return(repository.GetAvatarMetadataOutput{}, nil).
					Once()
				m.EXPECT().
					AddThumbnailKey(mock.Anything, mock.Anything).
					Return(nil).
					Times(len(_thumbnailSizes))
			},
			setupStorageMock: func(m *MockAvatarStorage, key string) {
				m.EXPECT().
					GetAvatar(mock.Anything, key).
					Return(storage.GetAvatarOutput{Bytes: dataJPEG}, nil).
					Once()
				m.EXPECT().
					PutAvatar(mock.Anything, mock.Anything).
					Return(storage.PutAvatarOutput{}, nil).
					Times(len(_thumbnailSizes))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := NewMockAvatarRepository(t)
			if tt.setupRepoMock != nil {
				tt.setupRepoMock(repo, tt.id, tt.key)
			}

			storage := NewMockAvatarStorage(t)
			if tt.setupStorageMock != nil {
				tt.setupStorageMock(storage, tt.key)
			}

			service := NewAvatarThumbnailService(repo, storage)
			require.NoError(t, service.GenerateThumbnail(context.Background(), tt.id))
		})
	}
}

func TestDeleteAvatar(t *testing.T) {
	dummyErr := errors.New("dummy")

	tests := []struct {
		name             string
		keys             []string
		wantErr          error
		setupStorageMock func(m *MockAvatarStorage, keys []string)
	}{
		{
			name: "success",
			keys: []string{"1", "2", "3"},
			setupStorageMock: func(m *MockAvatarStorage, keys []string) {
				for _, k := range keys {
					m.EXPECT().
						DeleteAvatar(mock.Anything, k).
						Return(nil).
						Once()
				}
			},
		},
		{
			name:    "storage error",
			keys:    []string{"1", "2", "3"},
			wantErr: dummyErr,
			setupStorageMock: func(m *MockAvatarStorage, keys []string) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, "1").
					Return(dummyErr).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := NewMockAvatarRepository(t)

			storage := NewMockAvatarStorage(t)
			if tt.setupStorageMock != nil {
				tt.setupStorageMock(storage, tt.keys)
			}

			service := NewAvatarThumbnailService(repo, storage)
			err := service.DeleteAvatar(context.Background(), tt.keys)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDecodeImage(t *testing.T) {
	var (
		dataJPEG        = echotest.LoadBytes(t, "../testdata/cat.jpeg")
		dataPNG         = echotest.LoadBytes(t, "../testdata/cat.png")
		dataWEBP        = echotest.LoadBytes(t, "../testdata/cat.webp")
		dataNotSupportd = []byte("not supported")
		dataPNGBroken   = bytes.Clone(dataPNG)[:len(dataPNG)-1]
	)

	tests := []struct {
		name       string
		data       []byte
		wantFormat string
		wantAnyErr bool
		wantErr    error
	}{
		{
			name:       "jpeg",
			data:       dataJPEG,
			wantFormat: "jpeg",
		},
		{
			name:       "png",
			data:       dataPNG,
			wantFormat: "png",
		},
		{
			name:       "webp",
			data:       dataWEBP,
			wantFormat: "webp",
		},
		{
			name:    "not supported",
			data:    dataNotSupportd,
			wantErr: image.ErrFormat,
		},
		{
			name:    "empty data",
			data:    nil,
			wantErr: image.ErrFormat,
		},
		{
			name:       "supported format but broken body",
			data:       dataPNGBroken,
			wantFormat: "png",
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img, format, err := decodeImage(tt.data)

			switch {
			case tt.wantErr != nil:
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, img)
			case tt.wantAnyErr:
				require.Error(t, err)
				require.Nil(t, img)
			default:
				require.NoError(t, err)
				require.NotNil(t, img)
			}

			require.Equal(t, tt.wantFormat, format)
		})
	}
}

func TestScaleImage(t *testing.T) {
	dataJPEG := echotest.LoadBytes(t, "../testdata/cat.jpeg")

	tests := []struct {
		name   string
		data   []byte
		width  int
		height int
	}{
		{
			name:   "success",
			data:   dataJPEG,
			width:  100,
			height: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			img, _, err := image.Decode(bytes.NewReader(tt.data))
			require.NoError(t, err)

			scaled := scaleImage(img, tt.width, tt.height)

			require.NotNil(t, scaled)
			require.Equal(t, tt.width, scaled.Bounds().Dx())
			require.Equal(t, tt.height, scaled.Bounds().Dy())
		})
	}
}

func TestEncodeImage(t *testing.T) {
	var (
		dataJPEG = echotest.LoadBytes(t, "../testdata/cat.jpeg")
		dataPNG  = echotest.LoadBytes(t, "../testdata/cat.png")
		dataWEBP = echotest.LoadBytes(t, "../testdata/cat.webp")
	)

	tests := []struct {
		name    string
		data    []byte
		format  string
		wantErr error
	}{
		{
			name:   "jpeg",
			data:   dataJPEG,
			format: "jpeg",
		},
		{
			name:   "png",
			data:   dataPNG,
			format: "png",
		},
		{
			name:   "webp",
			data:   dataWEBP,
			format: "webp",
		},
		{
			name:    "not supported",
			format:  "gif",
			wantErr: ErrFormatNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				img image.Image
				err error
			)
			if len(tt.data) != 0 {
				img, _, err = image.Decode(bytes.NewReader(tt.data))
				require.NoError(t, err)
			}

			var buf bytes.Buffer
			err = encodeImage(&buf, img, tt.format)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			_, format, err := image.DecodeConfig(&buf)
			require.NoError(t, err)
			require.Equal(t, tt.format, format)
		})
	}
}

func TestThumbnailFilename(t *testing.T) {
	tests := []struct {
		name           string
		avatarFilename string
		width          int
		height         int
		wantFilename   string
	}{
		{
			name:           "success",
			avatarFilename: "cat.jpeg",
			width:          100,
			height:         200,
			wantFilename:   "cat_100x200.jpeg",
		},
		{
			name:           "no extension",
			avatarFilename: "cat",
			width:          100,
			height:         200,
			wantFilename:   "cat_100x200",
		},
		{
			name:           "empty",
			avatarFilename: "",
			width:          100,
			height:         200,
			wantFilename:   "_100x200",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			filename := thumbnailFilename(tt.avatarFilename, tt.width, tt.height)
			require.Equal(t, tt.wantFilename, filename)
		})
	}
}
