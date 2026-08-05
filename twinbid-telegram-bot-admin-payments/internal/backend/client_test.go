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

	if gotPath != "/api/internal/campaigns/campaign-1/moderation" {
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

type presetTokenStore struct {
	tokens tokenstore.Tokens
}

func (s *presetTokenStore) Load() (tokenstore.Tokens, error) { return s.tokens, nil }
func (s *presetTokenStore) Save(tokens tokenstore.Tokens) error {
	s.tokens = tokens
	return nil
}

func TestTransactionAdminActionsUseJWTAndBotSecret(t *testing.T) {
	tests := []struct {
		name string
		path string
		call func(*Client) error
	}{
		{
			name: "approve",
			path: "/api/transactions/transaction-row-1/approve_admin",
			call: func(client *Client) error {
				return client.ApproveTransaction(context.Background(), "target-user", "transaction-row-1")
			},
		},
		{
			name: "cancel",
			path: "/api/transactions/transaction-row-1/cancel_admin",
			call: func(client *Client) error {
				return client.CancelTransaction(context.Background(), "target-user", "transaction-row-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotAuthorization, gotSecret, gotUserID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAuthorization = r.Header.Get("Authorization")
				gotSecret = r.Header.Get("X-Bot-Secret")
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				gotUserID = body["user_id"]
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":true,"errorMsg":"","data":{}}`))
			}))
			defer server.Close()

			store := &presetTokenStore{tokens: tokenstore.Tokens{
				AccessToken:  "admin-access-token",
				RefreshToken: "admin-refresh-token",
			}}
			client := NewClient(&config.Config{
				BackendBaseURL: server.URL,
				InternalSecret: "shared-secret",
			}, store)

			if err := tt.call(client); err != nil {
				t.Fatalf("admin transaction action error = %v", err)
			}
			if gotPath != tt.path {
				t.Fatalf("path = %q, want %q", gotPath, tt.path)
			}
			if gotAuthorization != "Bearer admin-access-token" {
				t.Fatalf("Authorization = %q", gotAuthorization)
			}
			if gotSecret != "shared-secret" {
				t.Fatalf("X-Bot-Secret = %q", gotSecret)
			}
			if gotUserID != "target-user" {
				t.Fatalf("user_id = %q", gotUserID)
			}
		})
	}
}
