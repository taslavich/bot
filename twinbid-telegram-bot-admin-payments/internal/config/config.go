package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken string
	HTTPAddr         string
	InternalSecret   string

	BackendBaseURL       string
	BackendAdminEmail    string
	BackendAdminPassword string

	TokenStorePath         string
	ChatStorePath          string
	PaymentActionStorePath string

	AllowedTelegramUserIDs map[int64]struct{}
}

func Load() (Config, error) {
	cfg := Config{
		TelegramBotToken:       env("TELEGRAM_BOT_TOKEN", ""),
		HTTPAddr:               env("HTTP_ADDR", ":8090"),
		InternalSecret:         env("INTERNAL_SECRET", ""),
		BackendBaseURL:         strings.TrimRight(env("BACKEND_BASE_URL", "https://twinbid.io"), "/"),
		BackendAdminEmail:      env("BACKEND_ADMIN_EMAIL", ""),
		BackendAdminPassword:   env("BACKEND_ADMIN_PASSWORD", ""),
		TokenStorePath:         env("TOKEN_STORE_PATH", "./data/tokens.json"),
		ChatStorePath:          env("CHAT_STORE_PATH", "./data/chats.json"),
		PaymentActionStorePath: env("PAYMENT_ACTION_STORE_PATH", "./data/payment_actions.json"),
	}

	cfg.AllowedTelegramUserIDs = parseAllowedUsers(env("ALLOWED_TELEGRAM_USER_IDS", ""))

	if cfg.TelegramBotToken == "" {
		return Config{}, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.BackendBaseURL == "" {
		return Config{}, fmt.Errorf("BACKEND_BASE_URL is required")
	}
	if cfg.BackendAdminEmail == "" || cfg.BackendAdminPassword == "" {
		return Config{}, fmt.Errorf("BACKEND_ADMIN_EMAIL and BACKEND_ADMIN_PASSWORD are required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func parseAllowedUsers(raw string) map[int64]struct{} {
	out := make(map[int64]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err == nil {
			out[id] = struct{}{}
		}
	}
	return out
}
