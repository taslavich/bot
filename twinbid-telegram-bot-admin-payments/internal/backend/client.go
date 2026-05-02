package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/tokenstore"
)

type TokenStore interface {
	Load() (tokenstore.Tokens, error)
	Save(tokenstore.Tokens) error
}

type Client struct {
	baseURL  string
	email    string
	password string
	store    TokenStore
	http     *http.Client
	mu       sync.Mutex
}

type apiEnvelope[T any] struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"errorMsg"`
	Data     T      `json:"data"`
}

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewClient(cfg *config.Config, store TokenStore) *Client {
	return &Client{
		baseURL:  strings.TrimRight(cfg.BackendBaseURL, "/"),
		email:    cfg.BackendAdminEmail,
		password: cfg.BackendAdminPassword,
		store:    store,
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) EnsureLoggedIn(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tokens, err := c.store.Load()
	if err != nil {
		return fmt.Errorf("load tokens: %w", err)
	}
	if tokens.AccessToken != "" && tokens.RefreshToken != "" {
		return nil
	}
	return c.loginLocked(ctx)
}

func (c *Client) PatchCampaignStatus(ctx context.Context, campaignID, status string) error {
	return c.doBusiness(ctx, http.MethodPatch, "/api/campaigns/"+campaignID, map[string]string{"status": status}, nil)
}

func (c *Client) ApproveTransaction(ctx context.Context, userID, id string) error {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" {
		return fmt.Errorf("user_id is required for transaction approve")
	}
	if id == "" {
		return fmt.Errorf("transaction row id is required for transaction approve")
	}
	return c.doBusiness(ctx, http.MethodPost, "/api/transactions/"+id+"/approve_admin", map[string]string{"user_id": userID}, nil)
}

func (c *Client) CancelTransaction(ctx context.Context, userID, id string) error {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" {
		return fmt.Errorf("user_id is required for transaction cancel")
	}
	if id == "" {
		return fmt.Errorf("transaction row id is required for transaction cancel")
	}
	return c.doBusiness(ctx, http.MethodPost, "/api/transactions/"+id+"/cancel_admin", map[string]string{"user_id": userID}, nil)
}

func (c *Client) PatchProfileBalanceIncrease(ctx context.Context, userID string, totalBalanceIncrease float64) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user_id is required for profile patch")
	}
	if totalBalanceIncrease <= 0 {
		return fmt.Errorf("total_balance_increase must be positive for profile patch")
	}

	body := map[string]any{
		"user_id": userID,
		"balance": totalBalanceIncrease,
	}
	return c.doBusiness(ctx, http.MethodPatch, "/api/profile_admin", body, nil)
}

func (c *Client) doBusiness(ctx context.Context, method, path string, body any, out any) error {
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return err
		}
	} else {
		raw = []byte(`{}`)
	}
	return c.doBusinessRaw(ctx, method, path, raw, out)
}

func (c *Client) doBusinessRaw(ctx context.Context, method, path string, rawBody []byte, out any) error {
	if err := c.EnsureLoggedIn(ctx); err != nil {
		return err
	}

	status, respBody, err := c.sendWithCurrentAccess(ctx, method, path, rawBody)
	if err != nil {
		return err
	}
	if status != http.StatusUnauthorized {
		return decodeBackendResponse(status, respBody, out)
	}

	if err := c.refreshOrLogin(ctx); err != nil {
		return err
	}
	status, respBody, err = c.sendWithCurrentAccess(ctx, method, path, rawBody)
	if err != nil {
		return err
	}
	return decodeBackendResponse(status, respBody, out)
}

func (c *Client) sendWithCurrentAccess(ctx context.Context, method, path string, rawBody []byte) (int, []byte, error) {
	tokens, err := c.store.Load()
	if err != nil {
		return 0, nil, fmt.Errorf("load tokens: %w", err)
	}
	return c.send(ctx, method, path, tokens.AccessToken, rawBody)
}

func (c *Client) send(ctx context.Context, method, path, accessToken string, rawBody []byte) (int, []byte, error) {
	var body io.Reader
	if method == http.MethodGet || method == http.MethodDelete {
		body = nil
	} else {
		body = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b, nil
}

func (c *Client) refreshOrLogin(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tokens, err := c.store.Load()
	if err != nil {
		return fmt.Errorf("load tokens: %w", err)
	}
	if tokens.RefreshToken != "" {
		if err := c.refreshLocked(ctx, tokens.RefreshToken); err == nil {
			return nil
		}
	}
	return c.loginLocked(ctx)
}

func (c *Client) loginLocked(ctx context.Context) error {
	payload := map[string]string{"email": c.email, "password": c.password}
	tokens, err := c.authRequest(ctx, "/api/auth/login", payload)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return c.store.Save(tokenstore.Tokens{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (c *Client) refreshLocked(ctx context.Context, refreshToken string) error {
	payload := map[string]string{"refresh_token": refreshToken}
	tokens, err := c.authRequest(ctx, "/api/auth/refresh", payload)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	return c.store.Save(tokenstore.Tokens{AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken})
}

func (c *Client) authRequest(ctx context.Context, path string, payload any) (authResponse, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return authResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return authResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return authResponse{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	var env apiEnvelope[authResponse]
	if err := json.Unmarshal(b, &env); err != nil {
		return authResponse{}, fmt.Errorf("status=%d body=%s", resp.StatusCode, string(b))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !env.Success {
		return authResponse{}, fmt.Errorf("status=%d error=%s body=%s", resp.StatusCode, env.ErrorMsg, string(b))
	}
	if env.Data.AccessToken == "" || env.Data.RefreshToken == "" {
		return authResponse{}, fmt.Errorf("empty tokens in response")
	}
	return env.Data, nil
}

func decodeBackendResponse(status int, body []byte, out any) error {
	if len(bytes.TrimSpace(body)) == 0 {
		if status >= 200 && status < 300 {
			return nil
		}
		return fmt.Errorf("backend status=%d empty body", status)
	}

	if out == nil {
		var env apiEnvelope[json.RawMessage]
		if err := json.Unmarshal(body, &env); err == nil {
			if status >= 200 && status < 300 && env.Success {
				return nil
			}
			return fmt.Errorf("backend status=%d error=%s body=%s", status, env.ErrorMsg, string(body))
		}
		if status >= 200 && status < 300 {
			return nil
		}
		return fmt.Errorf("backend status=%d body=%s", status, string(body))
	}

	var env apiEnvelope[json.RawMessage]
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("decode backend response: %w; status=%d body=%s", err, status, string(body))
	}
	if status < 200 || status >= 300 || !env.Success {
		return fmt.Errorf("backend status=%d error=%s body=%s", status, env.ErrorMsg, string(body))
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}
