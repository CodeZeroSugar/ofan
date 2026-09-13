package api

import (
	"html/template"
	"net/http"
	"net/http/httptest"
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

func TestRespondWithHTML(t *testing.T) {
	tests := []struct {
		name         string
		tmplName     string
		txtStr       string
		expectedCode int
		expectedStr  string
	}{
		{
			name:         "happy path",
			tmplName:     "greet",
			txtStr:       "Hello, {{.Name}}!",
			expectedCode: http.StatusOK,
			expectedStr:  "Hello, bob!",
		},
		{
			name:         "error path",
			tmplName:     "meet",
			txtStr:       "Hello, {{.Name}}!",
			expectedCode: http.StatusInternalServerError,
			expectedStr:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tmpl, err := template.New(tc.tmplName).Parse(tc.txtStr)
			require.NoError(t, err)
			dat := struct {
				Name string
				Msg  string
			}{
				Name: "bob",
				Msg:  "something",
			}
			respondWithHTML(rr, http.StatusOK, tmpl, "greet", dat)
			assert.Equal(t, tc.expectedCode, rr.Code)
			if tc.expectedCode != http.StatusOK {
				return
			}
			assert.Equal(t, "text/html; charset=utf-8", rr.Header().Get("Content-Type"))
			assert.Equal(t, tc.expectedStr, rr.Body.String())
		})
	}
}
