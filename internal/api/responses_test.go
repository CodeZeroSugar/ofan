package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWantsHtml(t *testing.T) {
	tests := []struct {
		name   string
		hValue string
		want   bool
	}{
		{
			name:   "absent",
			hValue: "",
			want:   false,
		},
		{
			name:   "anything",
			hValue: "*/*",
			want:   false,
		},
		{
			name:   "app/json",
			hValue: "application/json",
			want:   false,
		},
		{
			name:   "text/html",
			hValue: "text/html",
			want:   true,
		},
		{
			name:   "html/xhtml+xml",
			hValue: "text/html,application/xhtml+xml",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "www.fake-ofan.com", http.NoBody)
			require.NoError(t, err)
			req.Header.Set("Accept", tt.hValue)
			assert.Equal(t, tt.want, wantsHTML(req))
		})
	}
}
