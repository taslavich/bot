package tgbot

type CreativePayload struct {
	ID           string `json:"id"`
	CreativeName string `json:"creative_name"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Link         string `json:"link"`
	ImageURL     string `json:"image_url"`
	Width        *int   `json:"w"`
	Height       *int   `json:"h"`
}

type CampaignModerationRequest struct {
	ChatID       *int64            `json:"chat_id,omitempty"`
	CampaignID   string            `json:"campaign_id"`
	CampaignName string            `json:"campaign_name"`
	FormatType   string            `json:"format_type"`
	TrafficType  string            `json:"traffic_type"`
	QualityType  string            `json:"quality_type"`
	UserID       string            `json:"user_id"`
	UserEmail    string            `json:"user_email"`
	UserTelegram string            `json:"user_telegram"`
	Creatives    []CreativePayload `json:"creatives"`
}

type PaymentModerationRequest struct {
	ChatID               *int64  `json:"chat_id,omitempty"`
	ID                   string  `json:"id"`
	TransactionID        string  `json:"transaction_id"`
	UserID               string  `json:"user_id"`
	UserEmail            string  `json:"user_email"`
	UserTelegram         string  `json:"user_telegram"`
	PaymentMethod        string  `json:"payment_method"`
	DepositAmount        float64 `json:"deposit_amount"`
	BonusAmount          float64 `json:"bonus_amount"`
	TotalBalanceIncrease float64 `json:"total_balance_increase"`
	Currency             string  `json:"currency"`
	PromocodeID          string  `json:"promocode_id"`
	TransactionHash      string  `json:"transaction_hash"`
}
