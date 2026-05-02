# TwinBid Telegram moderation bot

Бот принимает от backend заявки на модерацию кампаний и платежей, отправляет их в Telegram-чат и по кнопкам вызывает backend TwinBid.

## Что умеет

### Кампании

Backend отправляет кампанию на:

```http
POST /internal/campaigns/moderation
X-Bot-Secret: <INTERNAL_SECRET>
```

Бот поддерживает 4 рекламных формата:

1. `popunder`
   - `format_type`
   - `traffic_type`
   - `campaign_name`
   - креативы: `creative_name`, `url`, `macros`

2. `banner`
   - `format_type`
   - `traffic_type`
   - `campaign_name`
   - `banner_size` или `w` + `h`
   - несколько креативов: `creative_name`, `url`, `macros`, `image_file` / `image_url`

3. `native`
   - `format_type`
   - `traffic_type`
   - `campaign_name`
   - `brand_name`
   - несколько креативов: `creative_name`, `url`, `macros`, `image_file` / `image_url`, `title`, `description`

4. `push`
   - `format_type`
   - `traffic_type`
   - `campaign_name`
   - `brand_name`
   - несколько креативов: `creative_name`, `url`, `macros`, `image_file` / `image_url`, `title`, `description`

Общие поля пользователя для всех 4 форматов кампаний:

```text
user_id      — обязательное поле
user_email   — обязательное поле, также поддерживается alias email
user_telegram — необязательное поле, также поддерживается alias telegram
```

В Telegram-сообщении эти поля выводятся отдельным блоком `Пользователь` для любого формата: popunder, banner, native, push.

Кнопки кампании:

```text
Одобрить  -> PATCH /api/campaigns/{campaign_id} {"status":"active"}
Отклонить -> PATCH /api/campaigns/{campaign_id} {"status":"draft"}
```

Статусы `active` и `draft` зашиты в коде, из `.env` не читаются.

### Платежи

Backend отправляет платеж на:

```http
POST /internal/payments/moderation
X-Bot-Secret: <INTERNAL_SECRET>
```

Кнопки платежа:

```text
Подтвердить -> POST /api/transactions/{id}/approve_admin {"user_id":"..."}
             -> PATCH /api/profile_admin {"user_id":"...", "balance": total_balance_increase}

Отклонить  -> POST /api/transactions/{id}/cancel_admin {"user_id":"..."}
```

`PATCH /api/profile_admin` после подтверждения платежа вызывается обязательно. Если он упал, бот пишет ошибку в Telegram и не считает действие успешным.

Поля платежа, которые бот принимает и выводит в Telegram:

```text
id                     — user_transactions.id, обязательное поле
transaction_id         — внешний id платежа, для отображения
user_id                — обязательное поле
user_email / email     — обязательное поле
user_telegram/telegram — необязательное поле
deposit_amount         — сумма пополнения; также поддерживается alias amount
total_balance_increase — конечная сумма начисления; также поддерживается alias final_amount
transaction_hash       — обязательное поле
```

Важно: `id` в платежном payload — это `user_transactions.id`, а не публичный `transaction_id`.

## Настройка

```bash
cp .env.example .env
nano .env
```

Минимально заполнить:

```env
TELEGRAM_BOT_TOKEN=...
BACKEND_BASE_URL=https://twinbid.io
BACKEND_ADMIN_EMAIL=...
BACKEND_ADMIN_PASSWORD=...
INTERNAL_SECRET=...
```

Для прода желательно заполнить:

```env
ALLOWED_TELEGRAM_USER_IDS=123456789,987654321
```

## Запуск

Локально:

```bash
go run ./cmd/bot
```

Через Docker:

```bash
docker compose up -d --build
```

## Команды в Telegram

Добавить бота в нужный чат и написать:

```text
/mode campaigns
```

или:

```text
/mode payments
```

Проверить chat_id:

```text
/chatid
```

Отключить чат:

```text
/mode off
```

## Пример JSON: popunder

```bash
curl -X POST "http://BOT_HOST:8090/internal/campaigns/moderation" \
  -H "Content-Type: application/json" \
  -H "X-Bot-Secret: change_me_long_random_string" \
  -d '{
    "campaign_id": "00000000-0000-0000-0000-000000000000",
    "format_type": "popunder",
    "traffic_type": "mainstream",
    "campaign_name": "Popunder campaign",
    "user_id": "11111111-1111-1111-1111-111111111111",
    "user_email": "user@example.com",
    "user_telegram": "@user",
    "creatives": [
      {
        "creative_name": "Pop creative 1",
        "url": "https://example.com/landing?click_id={click_id}",
        "macros": ["{click_id}", "{site_id}", "{campaign_id}"]
      }
    ]
  }'
```

## Пример JSON: banner

```json
{
  "campaign_id": "00000000-0000-0000-0000-000000000000",
  "format_type": "banner",
  "traffic_type": "mainstream",
  "campaign_name": "Banner campaign",
  "banner_size": "300x250",
  "user_id": "11111111-1111-1111-1111-111111111111",
  "user_email": "user@example.com",
  "user_telegram": "@user",
  "creatives": [
    {
      "creative_name": "Banner 1",
      "url": "https://example.com/landing?click_id={click_id}",
      "macros": {"click_id":"{click_id}", "site_id":"{site_id}"},
      "image_file": "https://cdn.example.com/banner-300x250.jpg"
    },
    {
      "creative_name": "Banner 2",
      "url": "https://example.com/landing2?click_id={click_id}",
      "macros": ["{click_id}", "{site_id}"],
      "image_url": "https://cdn.example.com/banner2-300x250.jpg"
    }
  ]
}
```

## Пример JSON: native

```json
{
  "campaign_id": "00000000-0000-0000-0000-000000000000",
  "format_type": "native",
  "traffic_type": "mainstream",
  "campaign_name": "Native campaign",
  "brand_name": "TwinBid",
  "user_id": "11111111-1111-1111-1111-111111111111",
  "user_email": "user@example.com",
  "user_telegram": "@user",
  "creatives": [
    {
      "creative_name": "Native creative 1",
      "url": "https://example.com/landing?click_id={click_id}",
      "macros": ["{click_id}", "{source}", "{site_id}"],
      "image_file": "https://cdn.example.com/native.jpg",
      "title": "Native title",
      "description": "Native description"
    }
  ]
}
```

## Пример JSON: push

```json
{
  "campaign_id": "00000000-0000-0000-0000-000000000000",
  "format_type": "push",
  "traffic_type": "mainstream",
  "campaign_name": "Push campaign",
  "brand_name": "TwinBid",
  "user_id": "11111111-1111-1111-1111-111111111111",
  "user_email": "user@example.com",
  "user_telegram": "@user",
  "creatives": [
    {
      "creative_name": "Push creative 1",
      "url": "https://example.com/landing?click_id={click_id}",
      "macros": ["{click_id}", "{source}", "{site_id}"],
      "image_file": "https://cdn.example.com/push-icon.jpg",
      "title": "Push title",
      "description": "Push description"
    }
  ]
}
```

## Пример JSON: payment

```bash
curl -X POST "http://BOT_HOST:8090/internal/payments/moderation" \
  -H "Content-Type: application/json" \
  -H "X-Bot-Secret: change_me_long_random_string" \
  -d '{
    "id": "33333333-3333-3333-3333-333333333333",
    "transaction_id": "public-transaction-id",
    "user_id": "11111111-1111-1111-1111-111111111111",
    "user_email": "user@example.com",
    "user_telegram": "@user",
    "payment_method": "usdt",
    "deposit_amount": 100,
    "bonus_amount": 25,
    "total_balance_increase": 125,
    "currency": "USD",
    "promocode_id": "TWINBID25",
    "transaction_hash": "0x..."
  }'
```

## Пример Go-интеграции backend -> bot

```go
package botnotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type BotClient struct {
	BaseURL        string
	InternalSecret string
	HTTP           *http.Client
}

func NewBotClient(baseURL, internalSecret string) *BotClient {
	return &BotClient{
		BaseURL:        strings.TrimRight(baseURL, "/"),
		InternalSecret: internalSecret,
		HTTP:           &http.Client{Timeout: 10 * time.Second},
	}
}

type BotCreative struct {
	CreativeName string `json:"creative_name"`
	URL          string `json:"url"`
	Macros       any    `json:"macros,omitempty"`
	ImageFile    string `json:"image_file,omitempty"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
}

type BotCampaignModerationRequest struct {
	CampaignID   string        `json:"campaign_id"`
	FormatType   string        `json:"format_type"`
	TrafficType  string        `json:"traffic_type"`
	CampaignName string        `json:"campaign_name"`
	BannerSize   string        `json:"banner_size,omitempty"`
	BrandName    string        `json:"brand_name,omitempty"`
	UserID       string        `json:"user_id"`
	UserEmail    string        `json:"user_email"`   // обязательно; можно отправлять alias `email`
	UserTelegram string        `json:"user_telegram"` // необязательно; можно отправлять alias `telegram`
	Creatives    []BotCreative `json:"creatives"`
}

func (c *BotClient) SendCampaignModeration(ctx context.Context, req BotCampaignModerationRequest) error {
	return c.postJSON(ctx, "/internal/campaigns/moderation", req)
}

func (c *BotClient) postJSON(ctx context.Context, path string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Bot-Secret", c.InternalSecret)

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("bot returned status %d", resp.StatusCode)
	}
	return nil
}
```

Вызов из backend после перевода кампании на модерацию:

```go
bot := botnotify.NewBotClient("http://127.0.0.1:8090", "change_me_long_random_string")

err := bot.SendCampaignModeration(ctx, botnotify.BotCampaignModerationRequest{
	CampaignID:   campaign.ID,
	FormatType:   campaign.FormatType,
	TrafficType:  campaign.TrafficType,
	CampaignName: campaign.CampaignName,
	BannerSize:   "300x250",
	BrandName:    valueOrEmpty(campaign.BrandName),
	UserID:       user.ID,
	UserEmail:    user.Mail,                  // обязательное поле
	UserTelegram: valueOrEmpty(user.Telegram), // необязательное поле
	Creatives: []botnotify.BotCreative{
		{
			CreativeName: creative.CreativeName,
			URL:          creative.URL,
			Macros:       []string{"{click_id}", "{site_id}"},
			ImageFile:    creative.ImageURL,
			Title:        creative.Title,
			Description:  creative.Description,
		},
	},
})
if err != nil {
	// лучше залогировать, но не ломать создание кампании, если Telegram временно недоступен
	log.Printf("send campaign moderation to telegram bot failed: %v", err)
}
```

## Авторизация в backend

При старте бот логинится через `POST /api/auth/login`, сохраняет `access_token` и `refresh_token` в `TOKEN_STORE_PATH`.

Перед каждым бизнес-вызовом отправляет:

```http
Authorization: Bearer <access_token>
```

Если получает `401`, вызывает `POST /api/auth/refresh`, сохраняет новую пару токенов и повторяет исходный запрос один раз. Если refresh не прошёл, делает login заново.
