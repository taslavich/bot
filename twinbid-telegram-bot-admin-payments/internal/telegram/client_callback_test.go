package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnswerCallbackQueryWithoutTextDoesNotRequestNotification(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/answerCallbackQuery" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, http: server.Client()}
	if err := client.AnswerCallbackQuery(context.Background(), "callback-1", "", false); err != nil {
		t.Fatalf("AnswerCallbackQuery: %v", err)
	}

	if got := payload["callback_query_id"]; got != "callback-1" {
		t.Fatalf("callback_query_id=%v", got)
	}
	if _, ok := payload["text"]; ok {
		t.Fatalf("empty callback answer must not contain text: %#v", payload)
	}
	if _, ok := payload["show_alert"]; ok {
		t.Fatalf("empty callback answer must not contain show_alert: %#v", payload)
	}
}
