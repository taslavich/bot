package tgbot

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"twinbid-telegram-bot/internal/backend"
)

func TestModerationConflictMessage(t *testing.T) {
	message, ok := moderationConflictMessage(&backend.APIError{
		Status:  http.StatusConflict,
		Message: "Модерация уже отменена пользователем",
	})
	if !ok || message != "Модерация уже отменена пользователем" {
		t.Fatalf("message=%q ok=%v", message, ok)
	}

	if _, ok := moderationConflictMessage(errors.New("network error")); ok {
		t.Fatal("network error must not be treated as moderation conflict")
	}
}

func TestValidateCampaignModerationVideo(t *testing.T) {
	req := CampaignModerationRequest{
		CampaignID:   "campaign-video",
		FormatType:   "video",
		TrafficType:  "mainstream",
		CampaignName: "Video campaign",
		UserID:       "user-1",
		UserEmail:    "user@example.com",
		Creatives: []CreativePayload{{
			CreativeName: "Video 1",
			ADM:          "https://example.com",
			ImageURL:     "https://cdn.example.com/video.mp4",
			VideoFormat:  "outstream_standard",
			VideoMetadata: &VideoCreativeMetadata{
				Duration: 30,
				Width:    1920,
				Height:   1080,
			},
		}},
	}

	if err := ValidateCampaignModeration(req); err != nil {
		t.Fatalf("ValidateCampaignModeration: %v", err)
	}
	if got := normalizeVideoFormat(req.Creatives[0].VideoFormat); got != "outstream" {
		t.Fatalf("normalizeVideoFormat=%q, want outstream", got)
	}
	text := campaignText(req)
	for _, want := range []string{"<b>format:</b> video", "<b>video_format:</b> outstream", "<b>video_duration:</b> 30s", "<b>video_dimensions:</b> 1920x1080"} {
		if !strings.Contains(text, want) {
			t.Fatalf("campaignText missing %q: %s", want, text)
		}
	}
}

func TestValidateCampaignModerationVideoRequiresVideoFormat(t *testing.T) {
	req := CampaignModerationRequest{
		CampaignID:   "campaign-video",
		FormatType:   "video",
		TrafficType:  "mainstream",
		CampaignName: "Video campaign",
		UserID:       "user-1",
		UserEmail:    "user@example.com",
		Creatives: []CreativePayload{{
			CreativeName: "Video 1",
			ADM:          "https://example.com",
			ImageURL:     "https://cdn.example.com/video.mp4",
		}},
	}

	if err := ValidateCampaignModeration(req); err == nil || !strings.Contains(err.Error(), "video_format is required") {
		t.Fatalf("error=%v, want video_format validation error", err)
	}
}

func TestValidateCampaignModerationRTBVideoDoesNotRequireCreative(t *testing.T) {
	req := CampaignModerationRequest{
		CampaignID:   "campaign-rtb-video",
		FormatType:   "video",
		TrafficType:  "mainstream",
		CampaignName: "RTB Video campaign",
		UserID:       "user-1",
		UserEmail:    "user@example.com",
		RTB:          true,
		DSPLink:      "https://dsp.example.com/openrtb",
	}
	if err := ValidateCampaignModeration(req); err != nil {
		t.Fatalf("ValidateCampaignModeration: %v", err)
	}
}
