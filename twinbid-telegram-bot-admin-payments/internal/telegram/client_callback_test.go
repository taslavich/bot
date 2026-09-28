package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestDeleteWebhookKeepsPendingUpdates(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%q", r.Method)
		}
		if r.URL.Path != "/deleteWebhook" {
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
	if err := client.DeleteWebhook(context.Background(), false); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}

	got, ok := payload["drop_pending_updates"]
	if !ok {
		t.Fatalf("drop_pending_updates is missing: %#v", payload)
	}
	if got != false {
		t.Fatalf("drop_pending_updates=%v, want false", got)
	}
}

func TestGetUpdatesExplicitlyRequestsCallbackQueries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method=%q", r.Method)
		}
		if r.URL.Path != "/getUpdates" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.URL.Query().Get("allowed_updates"); got != `["message","callback_query"]` {
			t.Fatalf("allowed_updates=%q", got)
		}
		if got := r.URL.Query().Get("timeout"); got != "30" {
			t.Fatalf("timeout=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, http: server.Client()}
	if _, err := client.GetUpdates(context.Background(), 0, 30); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
}

func TestWebhookActiveConflictIsClassified(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict: can't use getUpdates method while webhook is active; use deleteWebhook to delete the webhook first"}`))
	}))
	defer ts.Close()

	client := &Client{baseURL: ts.URL, http: ts.Client()}
	_, err := client.GetUpdates(context.Background(), 0, 30)
	if err == nil {
		t.Fatal("GetUpdates() error = nil, want webhook conflict")
	}
	if !IsWebhookActiveConflict(err) {
		t.Fatalf("IsWebhookActiveConflict(%v) = false, want true", err)
	}
}

func TestOtherConflictIsNotWebhookActiveConflict(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict: terminated by other getUpdates request"}`))
	}))
	defer ts.Close()

	client := &Client{baseURL: ts.URL, http: ts.Client()}
	_, err := client.GetUpdates(context.Background(), 0, 30)
	if err == nil {
		t.Fatal("GetUpdates() error = nil, want conflict")
	}
	if IsWebhookActiveConflict(err) {
		t.Fatalf("IsWebhookActiveConflict(%v) = true, want false", err)
	}
}

func TestSendVideoFileUsesTelegramVideoEndpoint(t *testing.T) {
	videoBytes := []byte("fake-mp4-data")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method=%q", r.Method)
		}
		if r.URL.Path != "/sendVideo" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("chat_id"); got != "123" {
			t.Fatalf("chat_id=%q", got)
		}
		if got := r.FormValue("supports_streaming"); got != "true" {
			t.Fatalf("supports_streaming=%q", got)
		}
		file, header, err := r.FormFile("video")
		if err != nil {
			t.Fatalf("FormFile(video): %v", err)
		}
		defer file.Close()
		if header.Filename != "creative.mp4" {
			t.Fatalf("filename=%q", header.Filename)
		}
		got, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if !bytes.Equal(got, videoBytes) {
			t.Fatalf("video bytes=%q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1,"chat":{"id":123}}}`))
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, http: server.Client()}
	if _, err := client.SendVideoFile(context.Background(), 123, "creative.mp4", "video/mp4", videoBytes, "caption", ModeHTML); err != nil {
		t.Fatalf("SendVideoFile: %v", err)
	}
}
