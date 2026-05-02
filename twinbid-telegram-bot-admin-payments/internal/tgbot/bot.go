package tgbot

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"twinbid-telegram-bot/internal/backend"
	"twinbid-telegram-bot/internal/config"
	"twinbid-telegram-bot/internal/telegram"
)

const (
	campaignApproveStatus = "active"
	campaignRejectStatus  = "draft"
)

type Bot struct {
	api            *telegram.Client
	modes          *ModeStore
	paymentActions *PaymentActionStore
	backend        *backend.Client
	cfg            config.Config
}

func New(cfg config.Config, backendClient *backend.Client, modes *ModeStore) (*Bot, error) {
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
	chats := b.targetChats(req.ChatID, ModeCampaigns)
	if len(chats) == 0 {
		return fmt.Errorf("no chats configured for campaigns mode")
	}

	for _, chatID := range chats {
		if err := b.sendCampaignToChat(ctx, chatID, req); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) SendPaymentModeration(ctx context.Context, req PaymentModerationRequest) error {
	chats := b.targetChats(req.ChatID, ModePayments)
	if len(chats) == 0 {
		return fmt.Errorf("no chats configured for payments mode")
	}

	action := PaymentAction{
		ID:                   req.ID,
		TransactionID:        req.TransactionID,
		UserID:               req.UserID,
		TotalBalanceIncrease: req.TotalBalanceIncrease,
	}
	key, err := b.paymentActions.Put(action)
	if err != nil {
		return err
	}

	for _, chatID := range chats {
		_, err := b.api.SendMessage(ctx, chatID, paymentText(req), telegram.ModeHTML, paymentKeyboard(key))
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) targetChats(explicit *int64, mode ChatMode) []int64 {
	if explicit != nil {
		return []int64{*explicit}
	}
	cfgs := b.modes.ListByMode(mode)
	out := make([]int64, 0, len(cfgs))
	for _, c := range cfgs {
		out = append(out, c.ChatID)
	}
	return out
}

func (b *Bot) sendCampaignToChat(ctx context.Context, chatID int64, req CampaignModerationRequest) error {
	for _, cr := range req.Creatives {
		if strings.TrimSpace(cr.ImageURL) == "" {
			continue
		}
		if _, err := b.api.SendPhoto(ctx, chatID, cr.ImageURL, creativeText(cr), telegram.ModeHTML); err != nil {
			log.Printf("send creative photo failed, fallback to text: %v", err)
			_, _ = b.api.SendMessage(ctx, chatID, "Не удалось отправить изображение Telegram, ссылка ниже:\n"+creativeText(cr), telegram.ModeHTML, nil)
		}
	}

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
	return nil
}

func (b *Bot) handleUpdate(ctx context.Context, upd telegram.Update) {
	if upd.Message != nil && isCommand(upd.Message.Text) {
		b.handleCommand(upd.Message)
		return
	}
	if upd.CallbackQuery != nil {
		b.handleCallback(ctx, upd.CallbackQuery)
		return
	}
}

func (b *Bot) handleCommand(m *telegram.Message) {
	cmd, args := parseCommand(m.Text)
	switch cmd {
	case "start", "help":
		b.reply(m.Chat.ID, helpText(), nil)
	case "chatid":
		b.reply(m.Chat.ID, fmt.Sprintf("chat_id: <code>%d</code>", m.Chat.ID), nil)
	case "mode":
		if m.From == nil || !b.isAllowed(m.From.ID) {
			b.reply(m.Chat.ID, "Нет прав на изменение режима.", nil)
			return
		}
		arg := strings.ToLower(strings.TrimSpace(args))
		if arg == "" {
			if c, ok := b.modes.Get(m.Chat.ID); ok {
				b.reply(m.Chat.ID, "Текущий режим чата: <b>"+html.EscapeString(string(c.Mode))+"</b>", nil)
			} else {
				b.reply(m.Chat.ID, "Для этого чата режим не задан. Используй <code>/mode campaigns</code> или <code>/mode payments</code>.", nil)
			}
			return
		}
		mode, ok := normalizeMode(arg)
		if !ok {
			b.reply(m.Chat.ID, "Неверный режим. Доступно: <code>campaigns</code>, <code>payments</code>, <code>off</code>.", nil)
			return
		}
		if err := b.modes.Set(m.Chat.ID, m.Chat.Title, mode); err != nil {
			b.reply(m.Chat.ID, "Ошибка сохранения режима: "+html.EscapeString(err.Error()), nil)
			return
		}
		if mode == ModeOff {
			b.reply(m.Chat.ID, "Режим для этого чата отключён.", nil)
		} else {
			b.reply(m.Chat.ID, "Готово. Этот чат теперь работает в режиме: <b>"+html.EscapeString(string(mode))+"</b>.", nil)
		}
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
		action, found := b.paymentActions.Get(id)
		if !found {
			b.answerCallback(q.ID, "Заявка не найдена. Возможно, бот перезапускался или кнопку уже нажимали.", true)
			return
		}
		err = b.backend.ApproveTransaction(ctx, action.UserID, action.ID)
		if err == nil {
			err = b.backend.PatchProfileBalanceIncrease(ctx, action.UserID, action.TotalBalanceIncrease)
		}
		if err == nil {
			_ = b.paymentActions.Delete(id)
		}
		okText = fmt.Sprintf("Платёж подтверждён, затем обязательно выполнен PATCH /api/profile. user_id=%s, transaction_row_id=%s, total_balance_increase=%.2f", action.UserID, action.ID, action.TotalBalanceIncrease)
	case "pay:no":
		action, found := b.paymentActions.Get(id)
		if !found {
			b.answerCallback(q.ID, "Заявка не найдена. Возможно, бот перезапускался или кнопку уже нажимали.", true)
			return
		}
		err = b.backend.CancelTransaction(ctx, action.UserID, action.ID)
		if err == nil {
			_ = b.paymentActions.Delete(id)
		}
		okText = "Платёж отклонён. user_id=" + action.UserID + ", transaction_row_id=" + action.ID
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
	var sb strings.Builder
	sb.WriteString("🟡 <b>Кампания на модерации</b>\n\n")
	line(&sb, "campaign_id", req.CampaignID)
	line(&sb, "campaign_name", req.CampaignName)
	line(&sb, "format_type", req.FormatType)
	line(&sb, "traffic_type", req.TrafficType)
	line(&sb, "quality_type", req.QualityType)
	sb.WriteString("\n<b>Пользователь</b>\n")
	line(&sb, "user_id", req.UserID)
	line(&sb, "email", req.UserEmail)
	line(&sb, "telegram", req.UserTelegram)

	sb.WriteString("\n<b>Креативы</b>\n")
	if len(req.Creatives) == 0 {
		sb.WriteString("— нет креативов в payload\n")
	}
	for i, cr := range req.Creatives {
		sb.WriteString("\n<b>Креатив #" + strconv.Itoa(i+1) + "</b>\n")
		creativeLines(&sb, cr)
	}
	return sb.String()
}

func creativeText(cr CreativePayload) string {
	var sb strings.Builder
	sb.WriteString("<b>Креатив</b>\n")
	creativeLines(&sb, cr)
	return sb.String()
}

func creativeLines(sb *strings.Builder, cr CreativePayload) {
	line(sb, "id", cr.ID)
	line(sb, "creative_name", cr.CreativeName)
	line(sb, "title", cr.Title)
	line(sb, "description", cr.Description)
	line(sb, "link", cr.Link)
	line(sb, "image_url", cr.ImageURL)
	if cr.Width != nil {
		line(sb, "w", strconv.Itoa(*cr.Width))
	}
	if cr.Height != nil {
		line(sb, "h", strconv.Itoa(*cr.Height))
	}
}

func paymentText(req PaymentModerationRequest) string {
	var sb strings.Builder
	sb.WriteString("🟡 <b>Платёж на модерации</b>\n\n")
	line(&sb, "id", req.ID)
	line(&sb, "transaction_id", req.TransactionID)
	line(&sb, "payment_method", req.PaymentMethod)
	line(&sb, "deposit_amount", fmt.Sprintf("%.2f %s", req.DepositAmount, req.Currency))
	line(&sb, "bonus_amount", fmt.Sprintf("%.2f", req.BonusAmount))
	line(&sb, "total_balance_increase", fmt.Sprintf("%.2f %s", req.TotalBalanceIncrease, req.Currency))
	line(&sb, "promocode_id", req.PromocodeID)
	line(&sb, "transaction_hash", req.TransactionHash)
	line(&sb, "user_id", req.UserID)
	line(&sb, "email", req.UserEmail)
	line(&sb, "telegram", req.UserTelegram)
	return sb.String()
}

func line(sb *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "—"
	}
	sb.WriteString("<b>" + html.EscapeString(key) + ":</b> " + html.EscapeString(value) + "\n")
}

func helpText() string {
	return strings.TrimSpace(`
Команды:
/mode campaigns — этот чат принимает кампании на модерацию
/mode payments — этот чат принимает платежи на модерацию
/mode off — отключить чат
/mode — показать текущий режим
/chatid — показать chat_id

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
