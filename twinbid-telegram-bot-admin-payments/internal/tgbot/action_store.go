package tgbot

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PaymentAction struct {
	Key                  string    `json:"key"`
	ID                   string    `json:"id"`
	TransactionID        string    `json:"transaction_id"`
	UserID               string    `json:"user_id"`
	TotalBalanceIncrease float64   `json:"total_balance_increase"`
	CreatedAt            time.Time `json:"created_at"`
}

type PaymentActionStore struct {
	mu    sync.Mutex
	path  string
	items map[string]PaymentAction
}

func NewPaymentActionStore(path string) (*PaymentActionStore, error) {
	s := &PaymentActionStore{path: path, items: map[string]PaymentAction{}}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PaymentActionStore) Put(action PaymentAction) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if action.ID == "" {
		return "", fmt.Errorf("payment action id is required")
	}
	if action.UserID == "" {
		return "", fmt.Errorf("payment action user_id is required")
	}
	if action.TotalBalanceIncrease <= 0 {
		return "", fmt.Errorf("payment action total_balance_increase must be positive")
	}

	key, err := randomKey(10)
	if err != nil {
		return "", err
	}
	action.Key = key
	action.CreatedAt = time.Now()
	s.items[key] = action
	if err := s.saveLocked(); err != nil {
		delete(s.items, key)
		return "", err
	}
	return key, nil
}

func (s *PaymentActionStore) Get(key string) (PaymentAction, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.items[key]
	return a, ok
}

func (s *PaymentActionStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return s.saveLocked()
}

func (s *PaymentActionStore) load() error {
	if s.path == "" {
		return nil
	}
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, &s.items)
}

func (s *PaymentActionStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o600)
}

func randomKey(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
