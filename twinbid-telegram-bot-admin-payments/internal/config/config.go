package config

import (
	"fmt"
	"log"
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

func getEnvFileNames() []string {
	return []string{".env.local", ".env", "api.env"}
}

func Load() (Config, error) {
	for _, fileName := range getEnvFileNames() {
		if err := loadDotEnvFile(fileName); err != nil {
			log.Printf("error loading %s: %v", fileName, err)
		}
	}

	cfg := Config{
		TelegramBotToken:       readString("TELEGRAM_BOT_TOKEN", "", true),
		HTTPAddr:               readString("HTTP_ADDR", ":8090", false),
		InternalSecret:         readString("INTERNAL_SECRET", "", false),
		BackendBaseURL:         strings.TrimRight(readString("BACKEND_BASE_URL", "https://twinbid.io", true), "/"),
		BackendAdminEmail:      readString("BACKEND_ADMIN_EMAIL", "", true),
		BackendAdminPassword:   readString("BACKEND_ADMIN_PASSWORD", "", true),
		TokenStorePath:         readString("TOKEN_STORE_PATH", "./data/tokens.json", false),
		ChatStorePath:          readString("CHAT_STORE_PATH", "./data/chats.json", false),
		PaymentActionStorePath: readString("PAYMENT_ACTION_STORE_PATH", "./data/payment_actions.json", false),
		AllowedTelegramUserIDs: parseAllowedUsers(readString("ALLOWED_TELEGRAM_USER_IDS", "", false)),
	}

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

func readString(key, fallback string, required bool) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	if required {
		return ""
	}
	return fallback
}

func loadDotEnvFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}
	return nil
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
