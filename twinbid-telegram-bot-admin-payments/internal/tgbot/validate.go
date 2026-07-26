package tgbot

import (
	"fmt"
	"strings"
)

func ValidateCampaignModeration(req CampaignModerationRequest) error {
	if strings.TrimSpace(req.CampaignID) == "" {
		return fmt.Errorf("campaign_id is required")
	}
	if strings.TrimSpace(req.FormatType) == "" {
		return fmt.Errorf("format_type is required")
	}
	if strings.TrimSpace(req.TrafficType) == "" {
		return fmt.Errorf("traffic_type is required")
	}
	if strings.TrimSpace(req.CampaignName) == "" {
		return fmt.Errorf("campaign_name is required")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(campaignUserEmail(req)) == "" {
		return fmt.Errorf("user_email or email is required")
	}
	if len(req.Creatives) == 0 {
		return fmt.Errorf("creatives must contain at least one creative")
	}

	format := normalizeFormat(req.FormatType)
	switch format {
	case "popunder", "banner", "native", "push":
		// supported
	default:
		return fmt.Errorf("unsupported format_type %q; allowed: popunder, banner, native, push", req.FormatType)
	}

	if format == "banner" && strings.TrimSpace(bannerSize(req)) == "" {
		return fmt.Errorf("banner_size or w+h is required for banner format")
	}

	for i, cr := range req.Creatives {
		prefix := fmt.Sprintf("creatives[%d]", i)
		if strings.TrimSpace(cr.CreativeName) == "" {
			return fmt.Errorf("%s.creative_name is required", prefix)
		}
		if strings.TrimSpace(cr.ADM) == "" {
			return fmt.Errorf("%s.adm is required", prefix)
		}
		switch format {
		case "banner":
		case "native", "push":
			if strings.TrimSpace(imageRef(cr)) == "" {
				return fmt.Errorf("%s.image_file or image_url is required for %s format", prefix, format)
			}
			if strings.TrimSpace(cr.Title) == "" {
				return fmt.Errorf("%s.title is required for %s format", prefix, format)
			}
			if strings.TrimSpace(cr.Description) == "" {
				return fmt.Errorf("%s.description is required for %s format", prefix, format)
			}
		}
	}
	return nil
}

func ValidatePaymentModeration(req PaymentModerationRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("id is required; this must be user_transactions.id, not public transaction_id")
	}
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(paymentUserEmail(req)) == "" {
		return fmt.Errorf("user_email or email is required")
	}
	if paymentDepositAmount(req) <= 0 {
		return fmt.Errorf("deposit_amount must be positive")
	}
	if paymentTotalBalanceIncrease(req) <= 0 {
		return fmt.Errorf("total_balance_increase or final_amount must be positive")
	}
	if strings.TrimSpace(req.TransactionHash) == "" {
		return fmt.Errorf("transaction_hash is required")
	}
	return nil
}

func ValidateTextMessage(req TextMessageRequest) error {
	if strings.TrimSpace(req.Text) == "" {
		return fmt.Errorf("text is required")
	}
	return nil
}
