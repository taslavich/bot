package tgbot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ChatMode string

const (
	ModeCampaigns ChatMode = "campaigns"
	ModePayments  ChatMode = "payments"
	ModeOff       ChatMode = "off"
)

type ChatConfig struct {
	ChatID    int64     `json:"chat_id"`
	Title     string    `json:"title"`
	Mode      ChatMode  `json:"mode"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ModeStore struct {
	path  string
	mu    sync.Mutex
	chats map[int64]ChatConfig
}

func NewModeStore(path string) (*ModeStore, error) {
	s := &ModeStore{path: path, chats: make(map[int64]ChatConfig)}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ModeStore) Set(chatID int64, title string, mode ChatMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if mode == ModeOff {
		delete(s.chats, chatID)
		return s.saveLocked()
	}
	s.chats[chatID] = ChatConfig{
		ChatID:    chatID,
		Title:     title,
		Mode:      mode,
		UpdatedAt: time.Now(),
	}
	return s.saveLocked()
}

func (s *ModeStore) Get(chatID int64) (ChatConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.chats[chatID]
	return c, ok
}

func (s *ModeStore) ListByMode(mode ChatMode) []ChatConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]ChatConfig, 0)
	for _, c := range s.chats {
		if c.Mode == mode {
			out = append(out, c)
		}
	}
	return out
}

func (s *ModeStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []ChatConfig
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, c := range list {
		s.chats[c.ChatID] = c
	}
	return nil
}

func (s *ModeStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	list := make([]ChatConfig, 0, len(s.chats))
	for _, c := range s.chats {
		list = append(list, c)
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
