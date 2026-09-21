package services

import (
	"context"
	"errors"
	"testing"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	dummyErr := errors.New("dummy")

	tests := []struct {
		name            string
		pingerCount     int
		wantErr         error
		setupPingerMock func(m *MockPinger)
	}{
		{
			name:        "success",
			pingerCount: 3,
			setupPingerMock: func(m *MockPinger) {
				m.EXPECT().
					Ping(mock.Anything).
					Return(nil).
					Once()
			},
		},
		{
			name:        "no pingers",
			pingerCount: 0,
		},
		{
			name:        "error",
			pingerCount: 3,
			wantErr:     dummyErr,
			setupPingerMock: func(m *MockPinger) {
				m.EXPECT().
					Ping(mock.Anything).
					Return(dummyErr).
					Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var pingers []Pinger
			for range tt.pingerCount {
				pinger := NewMockPinger(t)
				if tt.setupPingerMock != nil {
					tt.setupPingerMock(pinger)
				}
				pingers = append(pingers, pinger)
			}

			service := NewHealthService(pingers...)
			err := service.Ping(context.Background())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
