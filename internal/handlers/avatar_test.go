package handlers

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"uuid"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sudeeya/gophprofile/internal/domain"
	"github.com/sudeeya/gophprofile/internal/services"
)

func TestUploadAvatar(t *testing.T) {
	var (
		contentJPEG         = echotest.LoadBytes(t, "../testdata/cat.jpeg")
		contentPNG          = echotest.LoadBytes(t, "../testdata/cat.png")
		contentWEBP         = echotest.LoadBytes(t, "../testdata/cat.webp")
		contentNotSupported = []byte("not supported")
		contentTooLarge     = bytes.Repeat([]byte{'a'}, services.MaxAvatarSize+1)
	)

	tests := []struct {
		name           string
		config         echotest.ContextConfig
		userID         string
		wantStatusCode int
		setupMock      func(m *MockAvatarService)
	}{
		{
			name: "valid jpeg",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.jpeg",
							Content:   contentJPEG,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusCreated,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, nil).
					Once()
			},
		},
		{
			name: "valid png",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.png",
							Content:   contentPNG,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusCreated,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, nil).
					Once()
			},
		},
		{
			name: "valid webp",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.webp",
							Content:   contentWEBP,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusCreated,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, nil).
					Once()
			},
		},
		{
			name: "missing user id header",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.jpeg",
							Content:   contentJPEG,
						},
					},
				},
			},
			userID:         "",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "missing file",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "dummy",
							Filename:  "cat.jpeg",
							Content:   contentJPEG,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "avatar too large",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.jpeg",
							Content:   contentTooLarge,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusRequestEntityTooLarge,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, services.ErrAvatarTooLarge).
					Once()
			},
		},
		{
			name: "format not supported",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.jpeg",
							Content:   contentNotSupported,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusBadRequest,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, services.ErrFormatNotSupported).
					Once()
			},
		},
		{
			name: "internal service error",
			config: echotest.ContextConfig{
				MultipartForm: &echotest.MultipartForm{
					Files: []echotest.MultipartFormFile{
						{
							Fieldname: "file",
							Filename:  "cat.jpeg",
							Content:   contentJPEG,
						},
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusInternalServerError,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					UploadAvatar(mock.Anything, mock.Anything).
					Return(domain.Avatar{}, errors.New("dummy")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, rec := tt.config.ToContextRecorder(t)
			if tt.userID != "" {
				c.Request().Header.Set(UserIDHeaderKey, tt.userID)
			}

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.UploadAvatar(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestGetAvatar(t *testing.T) {
	tests := []struct {
		name           string
		id             uuid.UUID
		wantStatusCode int
		newConfig      func(id uuid.UUID) echotest.ContextConfig
		setupMock      func(m *MockAvatarService, id uuid.UUID)
	}{
		{
			name:           "success",
			id:             uuid.New(),
			wantStatusCode: http.StatusOK,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID) {
				m.EXPECT().
					GetAvatar(mock.Anything, id).
					Return(domain.Avatar{}, nil).
					Once()
			},
		},
		{
			name: "invalid id",
			newConfig: func(_ uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: "0",
						},
					},
				}
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "not found",
			id:             uuid.New(),
			wantStatusCode: http.StatusNotFound,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID) {
				m.EXPECT().
					GetAvatar(mock.Anything, id).
					Return(domain.Avatar{}, errors.New("not found")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, rec := tt.newConfig(tt.id).ToContextRecorder(t)

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service, tt.id)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.GetAvatar(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestGetAvatarMetadata(t *testing.T) {
	tests := []struct {
		name           string
		id             uuid.UUID
		wantStatusCode int
		newConfig      func(id uuid.UUID) echotest.ContextConfig
		setupMock      func(m *MockAvatarService, id uuid.UUID)
	}{
		{
			name:           "success",
			id:             uuid.New(),
			wantStatusCode: http.StatusOK,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID) {
				m.EXPECT().
					GetAvatarMetadata(mock.Anything, id).
					Return(domain.Metadata{}, nil).
					Once()
			},
		},
		{
			name:           "invalid id",
			wantStatusCode: http.StatusBadRequest,
			newConfig: func(_ uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: "0",
						},
					},
				}
			},
		},
		{
			name:           "not found",
			id:             uuid.New(),
			wantStatusCode: http.StatusNotFound,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID) {
				m.EXPECT().
					GetAvatarMetadata(mock.Anything, id).
					Return(domain.Metadata{}, errors.New("not found")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, rec := tt.newConfig(tt.id).ToContextRecorder(t)

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service, tt.id)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.GetAvatarMetadata(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestDeleteAvatar(t *testing.T) {
	tests := []struct {
		name           string
		id             uuid.UUID
		userID         string
		wantStatusCode int
		newConfig      func(id uuid.UUID) echotest.ContextConfig
		setupMock      func(m *MockAvatarService, id uuid.UUID, userId string)
	}{
		{
			name:           "success",
			id:             uuid.New(),
			userID:         "user",
			wantStatusCode: http.StatusNoContent,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID, userId string) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, services.DeleteAvatarInput{
						ID:     id,
						UserID: userId,
					}).
					Return(nil).
					Once()
			},
		},
		{
			name:           "missing user id header",
			userID:         "",
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name:           "invalid id",
			wantStatusCode: http.StatusBadRequest,
			newConfig: func(_ uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: "0",
						},
					},
				}
			},
		},
		{
			name:           "forbidden",
			id:             uuid.New(),
			userID:         "user",
			wantStatusCode: http.StatusForbidden,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID, userId string) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, services.DeleteAvatarInput{
						ID:     id,
						UserID: userId,
					}).
					Return(services.ErrForbidden).
					Once()
			},
		},
		{
			name:           "internal service error",
			id:             uuid.New(),
			userID:         "user",
			wantStatusCode: http.StatusInternalServerError,
			newConfig: func(id uuid.UUID) echotest.ContextConfig {
				return echotest.ContextConfig{
					PathValues: echo.PathValues{
						echo.PathValue{
							Name:  "id",
							Value: id.String(),
						},
					},
				}
			},
			setupMock: func(m *MockAvatarService, id uuid.UUID, userId string) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, services.DeleteAvatarInput{
						ID:     id,
						UserID: userId,
					}).
					Return(errors.New("dummy")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := echotest.ContextConfig{}
			if tt.newConfig != nil {
				cfg = tt.newConfig(tt.id)
			}

			c, rec := cfg.ToContextRecorder(t)
			if tt.userID != "" {
				c.Request().Header.Set(UserIDHeaderKey, tt.userID)
			}

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service, tt.id, tt.userID)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.DeleteAvatar(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}
