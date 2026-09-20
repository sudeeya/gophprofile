package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v5/echotest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name           string
		wantStatusCode int
		setupMock      func(m *MockHealthService)
	}{
		{
			name:           "success",
			wantStatusCode: http.StatusOK,
			setupMock: func(m *MockHealthService) {
				m.EXPECT().
					Ping(mock.Anything).
					Return(nil).
					Once()
			},
		},
		{
			name:           "unhealthy",
			wantStatusCode: http.StatusServiceUnavailable,
			setupMock: func(m *MockHealthService) {
				m.EXPECT().
					Ping(mock.Anything).
					Return(errors.New("unhealthy")).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, rec := echotest.ContextConfig{}.ToContextRecorder(t)

			service := NewMockHealthService(t)
			if tt.setupMock != nil {
				tt.setupMock(service)
			}

			handler := NewHealthHandler(service)
			require.NoError(t, handler.Health(c))

			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}
