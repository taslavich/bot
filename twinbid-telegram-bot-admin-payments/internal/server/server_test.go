package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestDecodeCampaignModerationAcceptsVideoFields(t *testing.T) {
	body := `{"campaign_id":"campaign-video","format_type":"video","traffic_type":"mainstream","campaign_name":"Video campaign","user_id":"user-1","user_email":"user@example.com","rtb":false,"creatives":[{"creative_name":"Video 1","adm":"https://example.com","image_url":"https://cdn.example.com/video.mp4","video_format":"instream","video_metadata":{"mimes":["video/mp4"],"duration":30,"protocols":[2,3,7],"api":[],"battr":[],"bitrate":0,"linearity":1,"skippable":true,"width":1920,"height":1080,"codec":"","file_size":123456}}]}`
	req := httptest.NewRequest(http.MethodPost, "/internal/campaigns/moderation", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	got, err := decodeCampaignModeration(req)
	if err != nil {
		t.Fatalf("decodeCampaignModeration: %v", err)
	}
	if got.FormatType != "video" || len(got.Creatives) != 1 {
		t.Fatalf("decoded request = %#v", got)
	}
	creative := got.Creatives[0]
	if creative.VideoFormat != "instream" {
		t.Fatalf("video_format=%q", creative.VideoFormat)
	}
	if creative.VideoMetadata == nil || creative.VideoMetadata.Duration != 30 || creative.VideoMetadata.Width != 1920 || creative.VideoMetadata.Height != 1080 {
		t.Fatalf("video_metadata=%#v", creative.VideoMetadata)
	}
}
