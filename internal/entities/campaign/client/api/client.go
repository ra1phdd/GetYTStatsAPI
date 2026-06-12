package campaignapi_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	"getytstatsapi/internal/core/security/serviceauth"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	serviceID  string
	secret     string
}

type Error struct {
	StatusCode int
	Body       string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("api request failed with status %d: %s", e.StatusCode, e.Body)
}

func (e *Error) IsNotFound() bool {
	return e != nil && e.StatusCode == http.StatusNotFound
}

type Campaign struct {
	ID                 int64                `json:"id"`
	TelegramUserID     int64                `json:"telegram_user_id"`
	ChannelID          string               `json:"channel_id"`
	ChannelTitle       string               `json:"channel_title"`
	Keyword            string               `json:"keyword"`
	StartDate          string               `json:"start_date"`
	Timezone           string               `json:"timezone"`
	Status             string               `json:"status"`
	Title              string               `json:"title"`
	TargetViews        *int64               `json:"target_views,omitempty"`
	Columns            []domain.StatsColumn `json:"columns,omitempty"`
	LastSnapshotAt     *time.Time           `json:"last_snapshot_at,omitempty"`
	LastTotalViews     int64                `json:"last_total_views"`
	LastDailyGrowth    *int64               `json:"last_daily_growth,omitempty"`
	EstimatedCloseDate string               `json:"estimated_close_date,omitempty"`
	ExportURL          string               `json:"export_url"`
	ExportToken        string               `json:"export_token"`
	SpreadsheetURL     string               `json:"spreadsheet_url,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	ClosedAt           *time.Time           `json:"closed_at,omitempty"`
	CloseReason        string               `json:"close_reason,omitempty"`
}

type CampaignList struct {
	Items    []Campaign `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasPrev  bool       `json:"has_prev"`
	HasNext  bool       `json:"has_next"`
	Total    int        `json:"total"`
}

type RefreshCampaignResponse struct {
	Campaign   Campaign                `json:"campaign"`
	Snapshot   domain.CampaignSnapshot `json:"snapshot"`
	AutoClosed bool                    `json:"auto_closed"`
}

type ChannelVerification struct {
	ChannelID        string `json:"channel_id"`
	ChannelTitle     string `json:"channel_title"`
	VerificationCode string `json:"verification_code"`
}

type GoogleLinkResponse struct {
	URL string `json:"url"`
}

func New(baseURL string, serviceID string, secret string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 20 * time.Second},
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		serviceID:  strings.TrimSpace(serviceID),
		secret:     strings.TrimSpace(secret),
	}
}

func (c *Client) ListChannels(ctx context.Context, userID int64) ([]domain.UserChannel, error) {
	var response struct {
		Items []domain.UserChannel `json:"items"`
	}
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d/channels", userID), nil, &response); err != nil {
		return nil, err
	}
	return response.Items, nil
}

func (c *Client) AddChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	var response domain.UserChannel
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/channels", userID), map[string]string{"channel_id": channelID}, &response)
	return response, err
}

func (c *Client) ResolveChannel(ctx context.Context, userID int64, channelRef string) (ChannelVerification, error) {
	var response ChannelVerification
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/channels/resolve", userID), map[string]string{"channel_id": channelRef}, &response)
	return response, err
}

func (c *Client) VerifyChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	var response domain.UserChannel
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/channels/verify", userID), map[string]string{"channel_id": channelID}, &response)
	return response, err
}

func (c *Client) GetGoogleLink(ctx context.Context, userID int64) (string, error) {
	var response GoogleLinkResponse
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/google/link", userID), nil, &response)
	return response.URL, err
}

func (c *Client) DeleteChannel(ctx context.Context, userID int64, channelID string) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/v1/users/%d/channels/%s", userID, url.PathEscape(channelID)), nil, nil)
}

func (c *Client) ListCampaigns(ctx context.Context, userID int64, status string, page int, pageSize int) (CampaignList, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = domain.DefaultCampaignsPageSize
	}
	query := url.Values{}
	if strings.TrimSpace(status) != "" {
		query.Set("status", status)
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("page_size", strconv.Itoa(pageSize))
	var response CampaignList
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d/campaigns?%s", userID, query.Encode()), nil, &response)
	return response, err
}

func (c *Client) CreateCampaign(ctx context.Context, userID int64, channelID string, keyword string, startDate string, targetViews *int64, columns []domain.StatsColumn) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/campaigns", userID), map[string]any{
		"channel_id":   channelID,
		"keyword":      keyword,
		"start_date":   startDate,
		"target_views": targetViews,
		"columns":      columns,
	}, &response)
	return response, err
}

func (c *Client) GetCampaign(ctx context.Context, userID int64, campaignID int64) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d/campaigns/%d", userID, campaignID), nil, &response)
	return response, err
}

func (c *Client) CloseCampaign(ctx context.Context, userID int64, campaignID int64) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/campaigns/%d/close", userID, campaignID), nil, &response)
	return response, err
}

func (c *Client) RefreshCampaign(ctx context.Context, userID int64, campaignID int64) (RefreshCampaignResponse, error) {
	var response RefreshCampaignResponse
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/campaigns/%d/refresh", userID, campaignID), nil, &response)
	return response, err
}

func (c *Client) UpdateCampaignColumns(ctx context.Context, userID int64, campaignID int64, columns []domain.StatsColumn) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/v1/users/%d/campaigns/%d/columns", userID, campaignID), map[string]any{
		"columns": columns,
	}, &response)
	return response, err
}

func (c *Client) UpdateCampaignTarget(ctx context.Context, userID int64, campaignID int64, targetViews *int64) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/v1/users/%d/campaigns/%d/target", userID, campaignID), map[string]any{
		"target_views": targetViews,
	}, &response)
	return response, err
}

func (c *Client) CreateCampaignSpreadsheet(ctx context.Context, userID int64, campaignID int64) (Campaign, error) {
	var response Campaign
	err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/v1/users/%d/campaigns/%d/spreadsheet", userID, campaignID), nil, &response)
	return response, err
}

func (c *Client) GetSettings(ctx context.Context, userID int64) (domain.UserSettings, error) {
	var response domain.UserSettings
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d/settings", userID), nil, &response)
	return response, err
}

func (c *Client) UpdateSettings(ctx context.Context, settings domain.UserSettings) (domain.UserSettings, error) {
	var response domain.UserSettings
	err := c.doJSON(ctx, http.MethodPatch, fmt.Sprintf("/v1/users/%d/settings", settings.TelegramUserID), map[string]any{
		"notifications_enabled":         settings.NotificationsEnabled,
		"notification_time":             settings.NotificationTime,
		"notification_interval_minutes": settings.NotificationIntervalMinutes,
		"timezone":                      settings.Timezone,
	}, &response)
	return response, err
}

func (c *Client) GetInputSession(ctx context.Context, userID int64) (domain.CampaignInputSession, error) {
	var response domain.CampaignInputSession
	err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/users/%d/input-session", userID), nil, &response)
	return response, err
}

func (c *Client) UpsertInputSession(ctx context.Context, session domain.CampaignInputSession) (domain.CampaignInputSession, error) {
	var response domain.CampaignInputSession
	err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/v1/users/%d/input-session", session.TelegramUserID), map[string]any{
		"flow":       session.Flow,
		"step":       session.Step,
		"payload":    session.Payload,
		"expires_at": session.ExpiresAt,
	}, &response)
	return response, err
}

func (c *Client) DeleteInputSession(ctx context.Context, userID int64) error {
	return c.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/v1/users/%d/input-session", userID), nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method string, path string, requestBody any, responseBody any) error {
	var body []byte
	var err error
	if requestBody != nil {
		body, err = json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := serviceauth.SignRequest(req, c.serviceID, c.secret); err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return &Error{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	if responseBody == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}
