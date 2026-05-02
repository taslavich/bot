# TwinBid Telegram moderation bot

Бот принимает от backend заявки на модерацию кампаний и платежей, отправляет их в Telegram-чат и по кнопкам вызывает backend TwinBid.

## Что зашито в коде

Статусы кампаний больше не берутся из `.env`:

- одобрить кампанию -> `active`
- отклонить кампанию -> `draft`

Подтверждение платежа выполняется строго цепочкой:

1. `POST /api/transactions/{id}/approve_admin` с телом `{ "user_id": "..." }`
2. `PATCH /api/profile` с телом `{ "user_id": "...", "balance": total_balance_increase }`

Если второй вызов падает, бот считает действие ошибочным и пишет ошибку в Telegram.

Отклонение платежа выполняет:

- `POST /api/transactions/{id}/cancel_admin` с телом `{ "user_id": "..." }`

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

## Отправка кампании в бота

Backend вызывает:

```bash
curl -X POST "http://BOT_HOST:8090/internal/campaigns/moderation" \
  -H "Content-Type: application/json" \
  -H "X-Bot-Secret: change_me_long_random_string" \
  -d '{
    "campaign_id": "00000000-0000-0000-0000-000000000000",
    "campaign_name": "Test campaign",
    "format_type": "native",
    "traffic_type": "mainstream",
    "quality_type": "standard",
    "user_id": "11111111-1111-1111-1111-111111111111",
    "user_email": "user@example.com",
    "user_telegram": "@user",
    "creatives": [
      {
        "id": "22222222-2222-2222-2222-222222222222",
        "creative_name": "Creative 1",
        "title": "Title",
        "description": "Description",
        "link": "https://example.com",
        "image_url": "https://example.com/image.jpg",
        "w": 300,
        "h": 250
      }
    ]
  }'
```

Кнопки:

- `Одобрить` -> `PATCH /api/campaigns/{campaign_id}` с `{"status":"active"}`
- `Отклонить` -> `PATCH /api/campaigns/{campaign_id}` с `{"status":"draft"}`

## Отправка платежа в бота

Backend вызывает:

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

Важно: `id` — это `user_transactions.id`, который используется в `/api/transactions/{id}/approve_admin` и `/api/transactions/{id}/cancel_admin`. `transaction_id` — только для отображения в Telegram.

Кнопки:

- `Подтвердить` -> `approve_admin`, затем обязательно `PATCH /api/profile`
- `Отклонить` -> `cancel_admin`

## Авторизация в backend

При старте бот логинится через `POST /api/auth/login`, сохраняет `access_token` и `refresh_token` в `TOKEN_STORE_PATH`.

Перед каждым бизнес-вызовом отправляет:

```http
Authorization: Bearer <access_token>
```

Если получает `401`, вызывает `POST /api/auth/refresh`, сохраняет новую пару токенов и повторяет исходный запрос один раз. Если refresh не прошёл, делает login заново.
