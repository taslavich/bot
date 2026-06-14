package tgbot

// UploadedFile is attached by the HTTP server when campaign moderation is sent as multipart/form-data.
// It is not decoded from JSON because a real file cannot be represented as a normal JSON field.
type UploadedFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

type CreativePayload struct {
	ID             string        `json:"id,omitempty"`
	CreativeName   string        `json:"creative_name"`
	URL            string        `json:"url"`
	Link           string        `json:"link,omitempty"`
	Macros         string        `json:"macros,omitempty"`
	TrackersMacros string        `json:"trackers_macros,omitempty"`
	ImageFile      *UploadedFile `json:"-"`
	ImageURL       string        `json:"image_url,omitempty"`
	PresignedS3URL string        `json:"presigned_s3_url,omitempty"`
	Name           string        `json:"name,omitempty"`
	W              *int          `json:"w,omitempty"`
	H              *int          `json:"h,omitempty"`
	Title          string        `json:"title,omitempty"`
	Description    string        `json:"description,omitempty"`
}

type CampaignModerationRequest struct {
	ChatID       *int64            `json:"chat_id,omitempty"`
	CampaignID   string            `json:"campaign_id"`
	FormatType   string            `json:"format_type"`
	TrafficType  string            `json:"traffic_type"`
	CampaignName string            `json:"campaign_name"`
	BannerSize   string            `json:"banner_size,omitempty"`
	W            *int              `json:"w,omitempty"`
	H            *int              `json:"h,omitempty"`
	BrandName    string            `json:"brand_name,omitempty"`
	QualityType  string            `json:"quality_type,omitempty"`
	UserID       string            `json:"user_id"`
	UserEmail    string            `json:"user_email,omitempty"`
	Email        string            `json:"email,omitempty"`
	UserTelegram string            `json:"user_telegram,omitempty"`
	Telegram     string            `json:"telegram,omitempty"`
	Creatives    []CreativePayload `json:"creatives"`
}

type PaymentModerationRequest struct {
	ChatID               *int64  `json:"chat_id,omitempty"`
	ID                   string  `json:"id"`
	TransactionID        string  `json:"transaction_id"`
	UserID               string  `json:"user_id"`
	UserEmail            string  `json:"user_email,omitempty"`
	Email                string  `json:"email,omitempty"`
	UserTelegram         string  `json:"user_telegram,omitempty"`
	Telegram             string  `json:"telegram,omitempty"`
	PaymentMethod        string  `json:"payment_method"`
	DepositAmount        float64 `json:"deposit_amount"`
	Amount               float64 `json:"amount,omitempty"`
	BonusAmount          float64 `json:"bonus_amount"`
	TotalBalanceIncrease float64 `json:"total_balance_increase"`
	FinalAmount          float64 `json:"final_amount,omitempty"`
	Currency             string  `json:"currency"`
	PromocodeID          string  `json:"promocode_id"`
	TransactionHash      string  `json:"transaction_hash"`
}

// TextMessageRequest describes a plain text message that should be sent to the configured Telegram chat.
type TextMessageRequest struct {
	Text string `json:"text"`
}
