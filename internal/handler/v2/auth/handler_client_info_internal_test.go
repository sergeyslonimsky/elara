package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
		wantIP string
		wantUA string
	}{
		{
			name:   "no headers",
			header: http.Header{},
		},
		{
			name: "single X-Forwarded-For value",
			header: http.Header{
				"X-Forwarded-For": {"203.0.113.5"},
				"User-Agent":      {"curl/8.0"},
			},
			wantIP: "203.0.113.5",
			wantUA: "curl/8.0",
		},
		{
			name: "multiple X-Forwarded-For values takes first and trims spaces",
			header: http.Header{
				"X-Forwarded-For": {"203.0.113.5, 70.41.3.18, 150.172.238.178"},
			},
			wantIP: "203.0.113.5",
		},
		{
			name: "empty X-Forwarded-For leaves ip empty",
			header: http.Header{
				"X-Forwarded-For": {""},
				"User-Agent":      {"my-agent"},
			},
			wantUA: "my-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotIP, gotUA := clientInfo(tt.header)

			assert.Equal(t, tt.wantIP, gotIP)
			assert.Equal(t, tt.wantUA, gotUA)
		})
	}
}
