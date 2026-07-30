package backend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/tokenstore"
)

type memoryTokenStore struct{}

func (memoryTokenStore) Load() (tokenstore.Tokens, error) { return tokenstore.Tokens{}, nil }
func (memoryTokenStore) Save(tokenstore.Tokens) error     { return nil }

func TestModerateCampaignUsesInternalEndpointAndSecret(t *testing.T) {
	var gotPath, gotSecret, gotDecision string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSecret = r.Header.Get("X-Bot-Secret")
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotDecision = body["decision"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"errorMsg":"","data":{}}`))
	}))
	defer server.Close()

	client := NewClient(&config.Config{BackendBaseURL: server.URL, InternalSecret: "shared-secret"}, memoryTokenStore{})
	if err := client.ModerateCampaign(context.Background(), "campaign-1", "approve"); err != nil {
		t.Fatalf("ModerateCampaign() error = %v", err)
	}

	if gotPath != "/internal/campaigns/campaign-1/moderation" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotSecret != "shared-secret" {
		t.Fatalf("secret = %q", gotSecret)
	}
	if gotDecision != "approve" {
		t.Fatalf("decision = %q", gotDecision)
	}
}

func TestDecodeBackendResponseReturnsTypedConflict(t *testing.T) {
	err := decodeBackendResponse(http.StatusConflict, []byte(`{"success":false,"errorMsg":"Модерация уже отменена пользователем","data":null}`), nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if apiErr.Status != http.StatusConflict || apiErr.Message != "Модерация уже отменена пользователем" {
		t.Fatalf("error = %#v", apiErr)
	}
}
