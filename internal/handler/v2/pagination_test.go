package v2

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   int32
		want    int
		wantErr bool
		code    connect.Code
	}{
		{
			name:  "zero",
			input: 0,
			want:  0,
		},
		{
			name:  "positive",
			input: 100,
			want:  100,
		},
		{
			name:    "negative",
			input:   -1,
			wantErr: true,
			code:    connect.CodeInvalidArgument,
		},
		{
			name:    "too large",
			input:   100_001,
			wantErr: true,
			code:    connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeLimit(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.code, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   int32
		want    int
		wantErr bool
		code    connect.Code
	}{
		{
			name:  "zero",
			input: 0,
			want:  0,
		},
		{
			name:  "positive",
			input: 1000,
			want:  1000,
		},
		{
			name:    "negative",
			input:   -1,
			wantErr: true,
			code:    connect.CodeInvalidArgument,
		},
		{
			name:    "too large",
			input:   1_000_001,
			wantErr: true,
			code:    connect.CodeInvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeOffset(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, tt.code, connect.CodeOf(err))
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
