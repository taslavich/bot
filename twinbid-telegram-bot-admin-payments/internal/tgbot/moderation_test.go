package tgbot

import (
	"errors"
	"net/http"
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
