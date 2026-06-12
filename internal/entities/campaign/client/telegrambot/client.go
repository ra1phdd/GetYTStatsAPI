package telegrambot_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	"getytstatsapi/internal/core/security/serviceauth"
	campaign_service "getytstatsapi/internal/entities/campaign/service"
)

type Client struct {
	httpClient *http.Client
	webhookURL string
	serviceID  string
	secret     string
}

func New(webhookURL string, serviceID string, secret string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		webhookURL: strings.TrimRight(strings.TrimSpace(webhookURL), "/"),
		serviceID:  strings.TrimSpace(serviceID),
		secret:     strings.TrimSpace(secret),
	}
}

func (c *Client) SendDigest(ctx context.Context, digest domain.NotificationDigest) error {
	return c.send(ctx, domain.BotWebhookEvent{
		EventID:   fmt.Sprintf("digest:%d:%s", digest.UserID, digest.NotificationDate.UTC().Format(time.RFC3339)),
		CreatedAt: time.Now().UTC(),
		Type:      domain.BotWebhookEventNotificationDigest,
		Digest:    &digest,
	})
}

func (c *Client) SendAutoClosed(ctx context.Context, campaign domain.Campaign) error {
	closedAt := time.Now().UTC()
	if campaign.ClosedAt != nil {
		closedAt = campaign.ClosedAt.UTC()
	}
	return c.send(ctx, domain.BotWebhookEvent{
		EventID:   fmt.Sprintf("campaign:auto-closed:%d:%s", campaign.ID, closedAt.Format(time.RFC3339)),
		CreatedAt: time.Now().UTC(),
		Type:      domain.BotWebhookEventCampaignAutoClosed,
		Campaign:  &campaign,
	})
}

var _ campaign_service.Notifier = (*Client)(nil)

func (c *Client) send(ctx context.Context, event domain.BotWebhookEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal bot webhook event: %w", err)
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL+"/v1/internal/webhooks/campaign-events", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("build bot webhook request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		if err := serviceauth.SignRequest(req, c.serviceID, c.secret); err != nil {
			return fmt.Errorf("sign bot webhook request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			data, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("bot webhook returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		} else {
			lastErr = fmt.Errorf("send bot webhook request: %w", err)
		}

		if attempt == 2 {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	return lastErr
}
