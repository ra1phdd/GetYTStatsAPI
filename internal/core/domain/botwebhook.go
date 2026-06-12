package domain

import "time"

const (
	BotWebhookEventNotificationDigest = "notification.digest.ready"
	BotWebhookEventCampaignAutoClosed = "campaign.auto_closed"
)

type BotWebhookEvent struct {
	EventID   string              `json:"event_id"`
	CreatedAt time.Time           `json:"created_at"`
	Type      string              `json:"type"`
	Digest    *NotificationDigest `json:"digest,omitempty"`
	Campaign  *Campaign           `json:"campaign,omitempty"`
}
