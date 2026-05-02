package config

import (
	"context"
	"log"
	"strconv"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string `env:"TELEGRAM_BOT_TOKEN" env-required:"true"`
	HTTPAddr         string `env:"HTTP_ADDR" env-default:":8090"`

	InternalSecret string `env:"INTERNAL_SECRET" env-default:""`

	BackendBaseURL       string `env:"BACKEND_BASE_URL" env-default:"https://twinbid.io"`
	BackendAdminEmail    string `env:"BACKEND_ADMIN_EMAIL" env-required:"true"`
	BackendAdminPassword string `env:"BACKEND_ADMIN_PASSWORD" env-required:"true"`

	TokenStorePath         string `env:"TOKEN_STORE_PATH" env-default:"./data/tokens.json"`
	ChatStorePath          string `env:"CHAT_STORE_PATH" env-default:"./data/chats.json"`
	PaymentActionStorePath string `env:"PAYMENT_ACTION_STORE_PATH" env-default:"./data/payment_actions.json"`

	CampaignsChatID int64 `env:"CAMPAIGNS_CHAT_ID" env-default:"0"`
	PaymentsChatID  int64 `env:"PAYMENTS_CHAT_ID" env-default:"0"`

	AllowedTelegramUserIDs AllowedUserIDs `env:"ALLOWED_TELEGRAM_USER_IDS" env-default:""`
}

type AllowedUserIDs map[int64]struct{}

func getEnvFileNames() []string {
	return []string{".env.local", ".env", "api.env"}
}

func Load(ctx context.Context) (Config, error) {
	for _, fileName := range getEnvFileNames() {
		if err := godotenv.Load(fileName); err != nil {
			log.Printf("error loading %s: %v", fileName, err)
		}
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, err
	}

	cfg.BackendBaseURL = strings.TrimRight(cfg.BackendBaseURL, "/")

	return &cfg, nil
}

func (a *AllowedUserIDs) SetValue(raw string) error {
	out := make(map[int64]struct{})

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return err
		}

		out[id] = struct{}{}
	}

	*a = out
	return nil
}

func (a AllowedUserIDs) Contains(id int64) bool {
	if len(a) == 0 {
		return true
	}

	_, ok := a[id]
	return ok
}
