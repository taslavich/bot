package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/tgbot"
)

type Server struct {
	cfg config.Config
	bot *tgbot.Bot
}

func New(cfg config.Config, bot *tgbot.Bot) *Server {
	return &Server{cfg: cfg, bot: bot}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /internal/campaigns/moderation", s.withSecret(s.campaignModeration))
	mux.HandleFunc("POST /internal/payments/moderation", s.withSecret(s.paymentModeration))
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) campaignModeration(w http.ResponseWriter, r *http.Request) {
	var req tgbot.CampaignModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.CampaignID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("campaign_id is required"))
		return
	}
	if err := s.bot.SendCampaignModeration(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) paymentModeration(w http.ResponseWriter, r *http.Request) {
	var req tgbot.PaymentModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("id is required; this must be user_transactions.id, not public transaction_id"))
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("user_id is required"))
		return
	}
	if req.TotalBalanceIncrease <= 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("total_balance_increase must be positive"))
		return
	}
	if err := s.bot.SendPaymentModeration(r.Context(), req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) withSecret(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.InternalSecret != "" && r.Header.Get("X-Bot-Secret") != s.cfg.InternalSecret {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("invalid X-Bot-Secret"))
			return
		}
		next(w, r)
	}
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"success": false, "error": err.Error()})
}
