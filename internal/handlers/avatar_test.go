package handlers

import (
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
		contentJPEG         = echotest.LoadBytes(t, "testdata/cat.jpeg")
		contentPNG          = echotest.LoadBytes(t, "testdata/cat.png")
		contentWEBP         = echotest.LoadBytes(t, "testdata/cat.webp")
		contentNotSupported = []byte("not supported")
		contentTooLarge     = make([]byte, services.MaxAvatarSize+1)
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
	id := uuid.New()

	tests := []struct {
		name           string
		config         echotest.ContextConfig
		wantStatusCode int
		setupMock      func(m *MockAvatarService)
	}{
		{
			name: "success",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			wantStatusCode: http.StatusOK,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					GetAvatar(mock.Anything, id).
					Return(domain.Avatar{}, nil).
					Once()
			},
		},
		{
			name: "invalid id",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: "0",
					},
				},
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "not found",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			wantStatusCode: http.StatusNotFound,
			setupMock: func(m *MockAvatarService) {
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

			c, rec := tt.config.ToContextRecorder(t)

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.GetAvatar(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestGetAvatarMetadata(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name           string
		config         echotest.ContextConfig
		wantStatusCode int
		setupMock      func(m *MockAvatarService)
	}{
		{
			name: "success",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			wantStatusCode: http.StatusOK,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					GetAvatarMetadata(mock.Anything, id).
					Return(domain.Metadata{}, nil).
					Once()
			},
		},
		{
			name: "invalid id",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: "0",
					},
				},
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "not found",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			wantStatusCode: http.StatusNotFound,
			setupMock: func(m *MockAvatarService) {
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

			c, rec := tt.config.ToContextRecorder(t)

			service := NewMockAvatarService(t)
			if tt.setupMock != nil {
				tt.setupMock(service)
			}

			handler := NewAvatarHandler(service)
			require.NoError(t, handler.GetAvatarMetadata(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestDeleteAvatar(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name           string
		config         echotest.ContextConfig
		userID         string
		wantStatusCode int
		setupMock      func(m *MockAvatarService)
	}{
		{
			name: "success",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusNoContent,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, mock.Anything).
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
			name: "invalid id",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: "0",
					},
				},
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "forbidden",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusForbidden,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, mock.Anything).
					Return(services.ErrForbidden).
					Once()
			},
		},
		{
			name: "internal service error",
			config: echotest.ContextConfig{
				PathValues: echo.PathValues{
					echo.PathValue{
						Name:  "id",
						Value: id.String(),
					},
				},
			},
			userID:         "user",
			wantStatusCode: http.StatusInternalServerError,
			setupMock: func(m *MockAvatarService) {
				m.EXPECT().
					DeleteAvatar(mock.Anything, mock.Anything).
					Return(errors.New("dummy")).
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
			require.NoError(t, handler.DeleteAvatar(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}
