package tgbot

import (
	"context"
	"fmt"
	"html"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"twinbid-telegram-bot/internal/backend"
	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/telegram"
)

const (
	campaignApproveStatus = "waiting"
	campaignRejectStatus  = "draft"
)

type Bot struct {
	api            *telegram.Client
	modes          *ModeStore
	paymentActions *PaymentActionStore
	backend        *backend.Client
	cfg            *config.Config
}

func New(cfg *config.Config, backendClient *backend.Client, modes *ModeStore) (*Bot, error) {
	api, err := telegram.New(cfg.TelegramBotToken)
	if err != nil {
		return nil, err
	}
	paymentActions, err := NewPaymentActionStore(cfg.PaymentActionStorePath)
	if err != nil {
		return nil, err
	}
	return &Bot{api: api, modes: modes, paymentActions: paymentActions, backend: backendClient, cfg: cfg}, nil
}

func (b *Bot) StartPolling(ctx context.Context) error {
	log.Printf("telegram bot started as @%s", b.api.Self.Username)
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		updates, err := b.api.GetUpdates(ctx, offset, 30)
		if err != nil {
			log.Printf("get updates failed: %v", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, upd := range updates {
			if upd.UpdateID >= offset {
				offset = upd.UpdateID + 1
			}
			b.handleUpdate(ctx, upd)
		}
	}
}

func (b *Bot) SendCampaignModeration(ctx context.Context, req CampaignModerationRequest) error {
	chatID := b.cfg.CampaignsChatID
	if chatID == 0 {
		return fmt.Errorf("CAMPAIGNS_CHAT_ID is not configured")
	}
	return b.sendCampaignToChat(ctx, chatID, req)
}

func (b *Bot) SendPaymentModeration(ctx context.Context, req PaymentModerationRequest) error {
	chatID := b.cfg.PaymentsChatID
	if chatID == 0 {
		return fmt.Errorf("PAYMENTS_CHAT_ID is not configured")
	}

	action := PaymentAction{
		ID:                   req.ID,
		TransactionID:        req.TransactionID,
		UserID:               req.UserID,
		TotalBalanceIncrease: paymentTotalBalanceIncrease(req),
	}
	key, err := b.paymentActions.Put(action)
	if err != nil {
		return err
	}

	_, err = b.api.SendMessage(ctx, chatID, paymentText(req), telegram.ModeHTML, paymentKeyboard(key))
	return err
}

func (b *Bot) sendCampaignToChat(ctx context.Context, chatID int64, req CampaignModerationRequest) error {
	text := campaignText(req)
	chunks := splitTelegramText(text, 3800)

	for i, chunk := range chunks {
		var markup any
		if i == len(chunks)-1 {
			markup = campaignKeyboard(req.CampaignID)
		}

		if _, err := b.api.SendMessage(ctx, chatID, chunk, telegram.ModeHTML, markup); err != nil {
			return err
		}
	}

	for _, cr := range req.Creatives {
		caption := creativePhotoCaption(req, cr)

		if cr.ImageFile != nil {
			if _, err := b.api.SendPhotoFile(ctx, chatID, cr.ImageFile.Filename, cr.ImageFile.ContentType, cr.ImageFile.Data, caption, telegram.ModeHTML); err != nil {
				log.Printf("send creative uploaded photo failed: %v", err)
			}
			continue
		}

		photoURL := creativePhotoRef(cr)
		if photoURL == "" {
			continue
		}

		if err := b.sendPhotoFromURL(ctx, chatID, photoURL, caption); err != nil {
			log.Printf("download/send creative photo failed: %v", err)
		}
	}

	return nil
}

func (b *Bot) handleUpdate(ctx context.Context, upd telegram.Update) {
	if upd.Message != nil && isCommand(upd.Message.Text) {
		cmd, _ := parseCommand(upd.Message.Text)
		log.Printf("received command update_id=%d chat_id=%d user_id=%d command=/%s text=%q", upd.UpdateID, upd.Message.Chat.ID, upd.Message.From.ID, cmd, upd.Message.Text)
		if cmd != "chatid" && b.fixedChatsConfigured() && !b.isFixedChat(upd.Message.Chat.ID) {
			log.Printf("skip command update_id=%d chat_id=%d command=/%s: chat is not allowed by fixed chat routing (CAMPAIGNS_CHAT_ID=%d, PAYMENTS_CHAT_ID=%d)", upd.UpdateID, upd.Message.Chat.ID, cmd, b.cfg.CampaignsChatID, b.cfg.PaymentsChatID)
			return
		}
		b.handleCommand(upd.Message)
		return
	}
	if upd.CallbackQuery != nil {
		if qmsg := upd.CallbackQuery.Message; qmsg != nil && b.fixedChatsConfigured() && !b.isFixedChat(qmsg.Chat.ID) {
			log.Printf("skip callback update_id=%d chat_id=%d data=%q: chat is not allowed by fixed chat routing (CAMPAIGNS_CHAT_ID=%d, PAYMENTS_CHAT_ID=%d)", upd.UpdateID, qmsg.Chat.ID, upd.CallbackQuery.Data, b.cfg.CampaignsChatID, b.cfg.PaymentsChatID)
			return
		}
		log.Printf("received callback update_id=%d callback_id=%s from_user_id=%d data=%q", upd.UpdateID, upd.CallbackQuery.ID, upd.CallbackQuery.From.ID, upd.CallbackQuery.Data)
		b.handleCallback(ctx, upd.CallbackQuery)
		return
	}
	log.Printf("skip update_id=%d: no command or callback payload", upd.UpdateID)
}
func (b *Bot) handleCommand(m *telegram.Message) {
	cmd, _ := parseCommand(m.Text)
	log.Printf("handle command chat_id=%d user_id=%d command=/%s", m.Chat.ID, m.From.ID, cmd)
	switch cmd {
	case "start", "help":
		b.reply(m.Chat.ID, helpText(), nil)
	case "chatid":
		b.reply(m.Chat.ID, fmt.Sprintf("chat_id: <code>%d</code>", m.Chat.ID), nil)
	case "mode":
		b.reply(m.Chat.ID, "Маршрутизация больше не задаётся через /mode. Кампании всегда уходят в <code>CAMPAIGNS_CHAT_ID</code>, платежи — в <code>PAYMENTS_CHAT_ID</code> из .env.", nil)
	default:
		b.reply(m.Chat.ID, "Неизвестная команда. Используй <code>/help</code>.", nil)
	}
}
func (b *Bot) handleCallback(ctx context.Context, q *telegram.CallbackQuery) {
	if q.Message == nil {
		return
	}
	if !b.isAllowed(q.From.ID) {
		b.answerCallback(q.ID, "Нет прав на это действие", true)
		return
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 {
		b.answerCallback(q.ID, "Некорректная кнопка", true)
		return
	}
	entity, action, id := parts[0], parts[1], parts[2]

	var err error
	var okText string
	switch entity + ":" + action {
	case "cmp:ok":
		err = b.backend.PatchCampaignStatus(ctx, id, campaignApproveStatus)
		okText = "Кампания одобрена, статус: " + campaignApproveStatus
	case "cmp:no":
		err = b.backend.PatchCampaignStatus(ctx, id, campaignRejectStatus)
		okText = "Кампания отклонена, статус: " + campaignRejectStatus
	case "pay:ok":
		actionData, found := b.paymentActions.Get(id)
		if !found {
			actionData = PaymentAction{}
		}

		actionData = fillPaymentActionFromMessage(actionData, q.Message.Text)
		if strings.TrimSpace(actionData.UserID) == "" || strings.TrimSpace(actionData.ID) == "" {
			b.answerCallback(q.ID, "Не удалось определить user_id или id платежа", true)
			return
		}

		err = b.backend.ApproveTransaction(ctx, actionData.UserID, actionData.ID)
		if err == nil && found {
			_ = b.paymentActions.Delete(id)
		}
		okText = fmt.Sprintf("Платёж подтверждён. user_id=%s, transaction_row_id=%s", actionData.UserID, actionData.ID)
	case "pay:no":
		actionData, found := b.paymentActions.Get(id)
		if !found {
			actionData = PaymentAction{}
		}

		actionData = fillPaymentActionFromMessage(actionData, q.Message.Text)
		if strings.TrimSpace(actionData.UserID) == "" || strings.TrimSpace(actionData.ID) == "" {
			b.answerCallback(q.ID, "Не удалось определить user_id или id платежа", true)
			return
		}

		err = b.backend.CancelTransaction(ctx, actionData.UserID, actionData.ID)
		if err == nil && found {
			_ = b.paymentActions.Delete(id)
		}
		okText = "Платёж отклонён. user_id=" + actionData.UserID + ", transaction_row_id=" + actionData.ID
	default:
		b.answerCallback(q.ID, "Некорректная кнопка", true)
		return
	}

	if err != nil {
		b.answerCallback(q.ID, "Ошибка", true)
		b.reply(q.Message.Chat.ID, "❌ Ошибка действия: <code>"+html.EscapeString(err.Error())+"</code>", nil)
		return
	}

	b.answerCallback(q.ID, "Готово", false)
	b.removeButtons(q.Message.Chat.ID, q.Message.MessageID)
	b.reply(q.Message.Chat.ID, "✅ "+html.EscapeString(okText)+"\nID: <code>"+html.EscapeString(id)+"</code>", nil)
}

func (b *Bot) isAllowed(userID int64) bool {
	if len(b.cfg.AllowedTelegramUserIDs) == 0 {
		return true
	}
	_, ok := b.cfg.AllowedTelegramUserIDs[userID]
	return ok
}

func (b *Bot) fixedChatsConfigured() bool {
	return b.cfg.CampaignsChatID != 0 || b.cfg.PaymentsChatID != 0
}

func (b *Bot) isFixedChat(chatID int64) bool {
	return chatID != 0 && (chatID == b.cfg.CampaignsChatID || chatID == b.cfg.PaymentsChatID)
}

func (b *Bot) reply(chatID int64, text string, markup any) {
	if _, err := b.api.SendMessage(context.Background(), chatID, text, telegram.ModeHTML, markup); err != nil {
		log.Printf("telegram send failed: %v", err)
	}
}

func (b *Bot) answerCallback(callbackID, text string, alert bool) {
	if err := b.api.AnswerCallbackQuery(context.Background(), callbackID, text, alert); err != nil {
		log.Printf("answer callback failed: %v", err)
	}
}

func (b *Bot) removeButtons(chatID int64, messageID int) {
	empty := telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{}}
	if err := b.api.EditMessageReplyMarkup(context.Background(), chatID, messageID, empty); err != nil {
		log.Printf("remove buttons failed: %v", err)
	}
}

func normalizeMode(raw string) (ChatMode, bool) {
	switch raw {
	case "campaign", "campaigns", "camp", "кампании":
		return ModeCampaigns, true
	case "payment", "payments", "pay", "платежи":
		return ModePayments, true
	case "off", "none", "disable", "выкл":
		return ModeOff, true
	default:
		return "", false
	}
}

func campaignKeyboard(campaignID string) telegram.InlineKeyboardMarkup {
	return telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{
		{
			{Text: "✅ Одобрить", CallbackData: "cmp:ok:" + campaignID},
			{Text: "❌ Отклонить", CallbackData: "cmp:no:" + campaignID},
		},
	}}
}

func paymentKeyboard(actionKey string) telegram.InlineKeyboardMarkup {
	return telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{
		{
			{Text: "✅ Подтвердить", CallbackData: "pay:ok:" + actionKey},
			{Text: "❌ Отклонить", CallbackData: "pay:no:" + actionKey},
		},
	}}
}

func campaignText(req CampaignModerationRequest) string {
	format := normalizeFormat(req.FormatType)
	var sb strings.Builder
	sb.WriteString("🟡 <b>Кампания на модерации</b>\n\n")
	line(&sb, "campaign_id", req.CampaignID)
	line(&sb, "format", req.FormatType)
	line(&sb, "traffic_type", req.TrafficType)
	line(&sb, "campaign_name", req.CampaignName)

	switch format {
	case "banner":
		line(&sb, "banner_size", bannerSize(req))
	case "native", "push":
		line(&sb, "brand_name", req.BrandName)
	}
	if strings.TrimSpace(req.QualityType) != "" {
		line(&sb, "quality_type", req.QualityType)
	}

	sb.WriteString("\n<b>Пользователь</b>\n")
	line(&sb, "user_id", req.UserID)
	line(&sb, "email", campaignUserEmail(req))
	line(&sb, "telegram", campaignUserTelegram(req))

	sb.WriteString("\n<b>Креативы</b>\n")
	if len(req.Creatives) == 0 {
		sb.WriteString("— нет креативов в payload\n")
	}
	for i, cr := range req.Creatives {
		sb.WriteString("\n<b>Креатив #" + strconv.Itoa(i+1) + "</b>\n")
		creativeLines(&sb, format, cr)
	}
	return sb.String()
}

func creativeText(cr CreativePayload) string {
	var sb strings.Builder
	sb.WriteString("<b>Креатив</b>\n")
	creativeLines(&sb, "", cr)
	return sb.String()
}

func creativePhotoCaption(req CampaignModerationRequest, cr CreativePayload) string {
	var sb strings.Builder
	sb.WriteString("<b>Картинка креатива</b>\n")
	line(&sb, "campaign_name", req.CampaignName)
	line(&sb, "creative_name", cr.CreativeName)
	return sb.String()
}

func creativeLines(sb *strings.Builder, format string, cr CreativePayload) {
	line(sb, "creative_name", cr.CreativeName)
	line(sb, "url", creativeURL(cr))
	line(sb, "macros", creativeMacros(cr))

	switch format {
	case "popunder":
		// Для popunder нужны только имя креатива, url и макросы.
	case "banner":
		line(sb, "image_file", imageRef(cr))
	case "native", "push":
		line(sb, "image_file", imageRef(cr))
		line(sb, "title", cr.Title)
		line(sb, "description", cr.Description)
	default:
		if cr.ID != "" {
			line(sb, "id", cr.ID)
		}
		if imageRef(cr) != "" {
			line(sb, "image_file", imageRef(cr))
		}
		if cr.Title != "" {
			line(sb, "title", cr.Title)
		}
		if cr.Description != "" {
			line(sb, "description", cr.Description)
		}
	}
}

func normalizeFormat(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "popunder", "pop", "попандер":
		return "popunder"
	case "banner", "баннер":
		return "banner"
	case "native", "натив":
		return "native"
	case "push", "inpagepush", "inpage", "пуш":
		return "push"
	default:
		return s
	}
}

func bannerSize(req CampaignModerationRequest) string {
	if strings.TrimSpace(req.BannerSize) != "" {
		return req.BannerSize
	}
	if req.W != nil && req.H != nil {
		return strconv.Itoa(*req.W) + "x" + strconv.Itoa(*req.H)
	}
	return ""
}

func creativeURL(cr CreativePayload) string {
	if strings.TrimSpace(cr.URL) != "" {
		return cr.URL
	}
	return cr.Link
}

func imageRef(cr CreativePayload) string {
	if cr.ImageFile != nil && strings.TrimSpace(cr.ImageFile.Filename) != "" {
		return cr.ImageFile.Filename
	}
	if strings.TrimSpace(cr.ImageURL) != "" {
		return cr.ImageURL
	}
	if strings.TrimSpace(cr.PresignedS3URL) != "" {
		return cr.PresignedS3URL
	}
	return cr.Name
}

func creativePhotoRef(cr CreativePayload) string {
	v := strings.TrimSpace(cr.ImageURL)
	if v == "" {
		v = strings.TrimSpace(cr.PresignedS3URL)
	}
	low := strings.ToLower(v)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return v
	}
	return ""
}

func (b *Bot) sendPhotoFromURL(ctx context.Context, chatID int64, imageURL string, caption string) error {
	httpClient := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download image status=%d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 15*1024*1024))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("downloaded image is empty")
	}

	filename := imageFilenameFromURL(imageURL, contentType)
	_, err = b.api.SendPhotoFile(ctx, chatID, filename, contentType, data, caption, telegram.ModeHTML)
	return err
}

func imageFilenameFromURL(rawURL string, contentType string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		name := path.Base(u.Path)
		if name != "." && name != "/" && name != "" {
			return name
		}
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		switch mediaType {
		case "image/jpeg":
			return "creative.jpg"
		case "image/png":
			return "creative.png"
		case "image/webp":
			return "creative.webp"
		case "image/gif":
			return "creative.gif"
		}
	}

	return "creative-image"
}

func creativeMacros(cr CreativePayload) string {
	if strings.TrimSpace(cr.Macros) != "" {
		return cr.Macros
	}
	return cr.TrackersMacros
}

func paymentText(req PaymentModerationRequest) string {
	var sb strings.Builder
	sb.WriteString("🟡 <b>Платёж на модерации</b>\n\n")
	line(&sb, "id", req.ID)
	line(&sb, "transaction_id", req.TransactionID)
	line(&sb, "payment_method", req.PaymentMethod)
	line(&sb, "сумма пополнения", fmt.Sprintf("%.2f %s", paymentDepositAmount(req), req.Currency))
	line(&sb, "бонус", fmt.Sprintf("%.2f%%", req.BonusAmount))
	line(&sb, "конечная сумма начисления", fmt.Sprintf("%.2f %s", paymentTotalBalanceIncrease(req), req.Currency))
	line(&sb, "transaction_hash", req.TransactionHash)
	line(&sb, "promocode_id", req.PromocodeID)
	line(&sb, "user_id", req.UserID)
	line(&sb, "email", paymentUserEmail(req))
	line(&sb, "telegram", paymentUserTelegram(req))
	return sb.String()
}

func paymentDepositAmount(req PaymentModerationRequest) float64 {
	if req.DepositAmount > 0 {
		return req.DepositAmount
	}
	return req.Amount
}

func paymentTotalBalanceIncrease(req PaymentModerationRequest) float64 {
	if req.TotalBalanceIncrease > 0 {
		return req.TotalBalanceIncrease
	}
	return req.FinalAmount
}

func campaignUserEmail(req CampaignModerationRequest) string {
	if strings.TrimSpace(req.UserEmail) != "" {
		return req.UserEmail
	}
	return req.Email
}

func campaignUserTelegram(req CampaignModerationRequest) string {
	if strings.TrimSpace(req.UserTelegram) != "" {
		return req.UserTelegram
	}
	return req.Telegram
}

func paymentUserEmail(req PaymentModerationRequest) string {
	if strings.TrimSpace(req.UserEmail) != "" {
		return req.UserEmail
	}
	return req.Email
}

func paymentUserTelegram(req PaymentModerationRequest) string {
	if strings.TrimSpace(req.UserTelegram) != "" {
		return req.UserTelegram
	}
	return req.Telegram
}

func line(sb *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "—"
	}
	sb.WriteString("<b>" + html.EscapeString(key) + ":</b> " + html.EscapeString(value) + "\n")
}

func fillPaymentActionFromMessage(action PaymentAction, text string) PaymentAction {
	rowID, userID := parsePaymentIDsFromMessage(text)

	if strings.TrimSpace(action.ID) == "" {
		action.ID = rowID
	}
	if strings.TrimSpace(action.UserID) == "" {
		action.UserID = userID
	}

	return action
}

func parsePaymentIDsFromMessage(text string) (rowID string, userID string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))

		if strings.HasPrefix(line, "id:") {
			rowID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			continue
		}

		if strings.HasPrefix(line, "user_id:") {
			userID = strings.TrimSpace(strings.TrimPrefix(line, "user_id:"))
			continue
		}
	}

	return rowID, userID
}

func helpText() string {
	return strings.TrimSpace(`
Команды:
/chatid — показать chat_id текущей беседы
/mode — больше не используется для маршрутизации

Маршрутизация фиксированная:
CAMPAIGNS_CHAT_ID — беседа для кампаний
PAYMENTS_CHAT_ID — беседа для платежей

Кнопки под заявками дергают backend-ручки TwinBid с авторизацией через bot-admin.
`)
}

func splitTelegramText(s string, limit int) []string {
	if len(s) <= limit {
		return []string{s}
	}
	var out []string
	for len(s) > limit {
		cut := strings.LastIndex(s[:limit], "\n")
		if cut < 1000 {
			cut = limit
		}
		out = append(out, s[:cut])
		s = strings.TrimLeft(s[cut:], "\n")
	}
	if s != "" {
		out = append(out, s)
	}
	return out
}

func isCommand(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "/")
}

func parseCommand(text string) (cmd string, args string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", ""
	}
	text = strings.TrimPrefix(text, "/")
	parts := strings.SplitN(text, " ", 2)
	cmd = strings.ToLower(parts[0])
	if at := strings.Index(cmd, "@"); at >= 0 {
		cmd = cmd[:at]
	}
	if len(parts) == 2 {
		args = parts[1]
	}
	return cmd, args
}
