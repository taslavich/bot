package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"twinbid-telegram-bot/internal/config"
)

func TestWithSecretFailsClosed(t *testing.T) {
	tests := []struct {
		name           string
		configured     string
		provided       string
		wantStatusCode int
	}{
		{
			name:           "missing configured secret",
			configured:     "",
			provided:       "anything",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "missing request secret",
			configured:     "shared-secret",
			provided:       "",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "wrong request secret",
			configured:     "shared-secret",
			provided:       "wrong-secret",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "matching secret",
			configured:     "shared-secret",
			provided:       "shared-secret",
			wantStatusCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{cfg: &config.Config{InternalSecret: tt.configured}}
			handler := s.withSecret(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodPost, "/internal/test", nil)
			if tt.provided != "" {
				req.Header.Set("X-Bot-Secret", tt.provided)
			}
			resp := httptest.NewRecorder()
			handler(resp, req)

			if resp.Code != tt.wantStatusCode {
				t.Fatalf("status = %d, want %d", resp.Code, tt.wantStatusCode)
			}
		})
	}
}
