package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const ModeHTML = "HTML"

type Client struct {
	token   string
	baseURL string
	http    *http.Client
	Self    User
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	Caption   string `json:"caption"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int            `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	Description string `json:"description"`
}

func New(token string) (*Client, error) {
	c := &Client{
		token:   token,
		baseURL: "https://api.telegram.org/bot" + token,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
	self, err := c.GetMe(context.Background())
	if err != nil {
		return nil, err
	}
	c.Self = self
	return c, nil
}

func (c *Client) GetMe(ctx context.Context) (User, error) {
	return requestJSON[User](ctx, c, http.MethodGet, "/getMe", nil)
}

func (c *Client) GetUpdates(ctx context.Context, offset int, timeout int) ([]Update, error) {
	q := url.Values{}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	q.Set("timeout", strconv.Itoa(timeout))
	return requestJSON[[]Update](ctx, c, http.MethodGet, "/getUpdates?"+q.Encode(), nil)
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, parseMode string, replyMarkup any) (Message, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	return requestJSON[Message](ctx, c, http.MethodPost, "/sendMessage", payload)
}

func (c *Client) SendPhoto(ctx context.Context, chatID int64, photoURL string, caption string, parseMode string) (Message, error) {
	payload := map[string]any{
		"chat_id": chatID,
		"photo":   photoURL,
	}
	if caption != "" {
		payload["caption"] = caption
	}
	if parseMode != "" {
		payload["parse_mode"] = parseMode
	}
	return requestJSON[Message](ctx, c, http.MethodPost, "/sendPhoto", payload)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, callbackID string, text string, showAlert bool) error {
	payload := map[string]any{
		"callback_query_id": callbackID,
		"text":              text,
		"show_alert":        showAlert,
	}
	_, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, "/answerCallbackQuery", payload)
	return err
}

func (c *Client) EditMessageReplyMarkup(ctx context.Context, chatID int64, messageID int, replyMarkup any) error {
	payload := map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"reply_markup": replyMarkup,
	}
	_, err := requestJSON[json.RawMessage](ctx, c, http.MethodPost, "/editMessageReplyMarkup", payload)
	return err
}

func requestJSON[T any](ctx context.Context, c *Client, method, path string, payload any) (T, error) {
	var zero T
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return zero, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return zero, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	var env apiResponse[T]
	if err := json.Unmarshal(b, &env); err != nil {
		return zero, fmt.Errorf("telegram status=%d body=%s", resp.StatusCode, string(b))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !env.OK {
		return zero, fmt.Errorf("telegram status=%d description=%s body=%s", resp.StatusCode, env.Description, string(b))
	}
	return env.Result, nil
}
