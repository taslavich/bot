package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/tgbot"
)

const maxMultipartMemory = 32 << 20

type Server struct {
	cfg *config.Config
	bot *tgbot.Bot
}

func New(cfg *config.Config, bot *tgbot.Bot) *Server {
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
	req, err := decodeCampaignModeration(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := tgbot.ValidateCampaignModeration(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
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
	if err := tgbot.ValidatePaymentModeration(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
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

func decodeCampaignModeration(r *http.Request) (tgbot.CampaignModerationRequest, error) {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return decodeCampaignMultipart(r)
	}

	var req tgbot.CampaignModerationRequest
	if err := decodeJSON(r, &req); err != nil {
		return req, err
	}
	return req, nil
}

func decodeCampaignMultipart(r *http.Request) (tgbot.CampaignModerationRequest, error) {
	defer r.Body.Close()
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return tgbot.CampaignModerationRequest{}, err
	}

	rawPayload := strings.TrimSpace(r.FormValue("payload"))
	if rawPayload == "" {
		return tgbot.CampaignModerationRequest{}, fmt.Errorf("multipart field payload is required")
	}

	var req tgbot.CampaignModerationRequest
	dec := json.NewDecoder(strings.NewReader(rawPayload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, fmt.Errorf("decode payload: %w", err)
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		return req, nil
	}
	for i := range req.Creatives {
		fieldName := fmt.Sprintf("creative_image_%d", i)
		file, ok, err := readMultipartFile(r.MultipartForm, fieldName)
		if err != nil {
			return req, err
		}
		if ok {
			req.Creatives[i].ImageFile = &file
		}
	}
	return req, nil
}

func readMultipartFile(form *multipart.Form, fieldName string) (tgbot.UploadedFile, bool, error) {
	files := form.File[fieldName]
	if len(files) == 0 {
		return tgbot.UploadedFile{}, false, nil
	}
	fh := files[0]
	f, err := fh.Open()
	if err != nil {
		return tgbot.UploadedFile{}, false, fmt.Errorf("open multipart file %s: %w", fieldName, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return tgbot.UploadedFile{}, false, fmt.Errorf("read multipart file %s: %w", fieldName, err)
	}
	if len(data) == 0 {
		return tgbot.UploadedFile{}, false, fmt.Errorf("multipart file %s is empty", fieldName)
	}
	return tgbot.UploadedFile{Filename: fh.Filename, ContentType: fh.Header.Get("Content-Type"), Data: data}, true, nil
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
