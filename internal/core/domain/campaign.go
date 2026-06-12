package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	CampaignStatusDraft     = "draft"
	CampaignStatusScheduled = "scheduled"
	CampaignStatusActive    = "active"
	CampaignStatusClosed    = "closed"

	CampaignCloseReasonManual        = "manual"
	CampaignCloseReasonTargetReached = "target_reached"

	DefaultUserTimezone         = "Europe/Moscow"
	DefaultNotificationTime     = "12:00"
	DefaultCampaignsPageSize    = 7
	DefaultAccessTokenLifetime  = 15 * time.Minute
	DefaultRefreshTokenLifetime = 30 * 24 * time.Hour
)

type UserChannel struct {
	ID             int64     `json:"id"`
	TelegramUserID int64     `json:"telegram_user_id"`
	ChannelID      string    `json:"channel_id"`
	ChannelTitle   string    `json:"channel_title"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UserSettings struct {
	TelegramUserID         int64      `json:"telegram_user_id"`
	NotificationsEnabled   bool       `json:"notifications_enabled"`
	NotificationTime       string     `json:"notification_time"`
	Timezone               string     `json:"timezone"`
	GoogleEmail            string     `json:"google_email,omitempty"`
	GoogleRefreshToken     string     `json:"-"`
	GoogleConnectedAt      *time.Time `json:"google_connected_at,omitempty"`
	LastNotificationSentAt *time.Time `json:"last_notification_sent_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

func DefaultUserSettings(userID int64) UserSettings {
	return UserSettings{
		TelegramUserID:       userID,
		NotificationsEnabled: true,
		NotificationTime:     DefaultNotificationTime,
		Timezone:             DefaultUserTimezone,
	}
}

type Campaign struct {
	ID                 int64         `json:"id"`
	TelegramUserID     int64         `json:"telegram_user_id"`
	ChannelID          string        `json:"channel_id"`
	ChannelTitle       string        `json:"channel_title"`
	Keyword            string        `json:"keyword"`
	StartDate          time.Time     `json:"start_date"`
	Timezone           string        `json:"timezone"`
	TargetViews        *int64        `json:"target_views,omitempty"`
	Columns            []StatsColumn `json:"columns,omitempty"`
	Status             string        `json:"status"`
	ExportJWT          string        `json:"-"`
	ClosedAt           *time.Time    `json:"closed_at,omitempty"`
	CloseReason        string        `json:"close_reason,omitempty"`
	LastSnapshotAt     *time.Time    `json:"last_snapshot_at,omitempty"`
	LastTotalViews     int64         `json:"last_total_views"`
	LastDailyGrowth    *int64        `json:"last_daily_growth,omitempty"`
	EstimatedCloseDate *time.Time    `json:"estimated_close_date,omitempty"`
	SpreadsheetID      string        `json:"-"`
	SpreadsheetURL     string        `json:"spreadsheet_url,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

func (c Campaign) Title() string {
	parts := []string{c.Keyword, c.StartDate.Format("02.01.06")}
	if c.TargetViews != nil {
		parts = append(parts, FormatViewsTarget(*c.TargetViews))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func (c Campaign) ExportURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(c.ExportJWT) == "" {
		return ""
	}
	return baseURL + "/v1/campaigns/export/" + c.ExportJWT
}

type CampaignSnapshot struct {
	ID                 int64        `json:"id"`
	CampaignID         int64        `json:"campaign_id"`
	SnapshotAt         time.Time    `json:"snapshot_at"`
	TotalViews         int64        `json:"total_views"`
	RemainingViews     *int64       `json:"remaining_views,omitempty"`
	DailyGrowth        *int64       `json:"daily_growth,omitempty"`
	EstimatedCloseDate *time.Time   `json:"estimated_close_date,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	Videos             []StatsVideo `json:"videos,omitempty"`
}

type CampaignList struct {
	Items    []Campaign `json:"items"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasPrev  bool       `json:"has_prev"`
	HasNext  bool       `json:"has_next"`
	Total    int        `json:"total"`
}

type CampaignInputSession struct {
	TelegramUserID int64      `json:"telegram_user_id"`
	Flow           string     `json:"flow"`
	Step           string     `json:"step"`
	Payload        string     `json:"payload"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type PublicUserSession struct {
	ID               string     `json:"id"`
	TelegramUserID   int64      `json:"telegram_user_id"`
	RefreshTokenHash string     `json:"-"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type NotificationDigestCampaign struct {
	CampaignID           int64      `json:"campaign_id"`
	Title                string     `json:"title"`
	Status               string     `json:"status"`
	TotalViews           int64      `json:"total_views"`
	RemainingViews       *int64     `json:"remaining_views,omitempty"`
	DailyGrowth          *int64     `json:"daily_growth,omitempty"`
	EstimatedCloseDate   *time.Time `json:"estimated_close_date,omitempty"`
	EstimatedCloseInDays *int64     `json:"estimated_close_in_days,omitempty"`
	ExportURL            string     `json:"export_url,omitempty"`
}

type NotificationDigest struct {
	UserID           int64                        `json:"user_id"`
	NotificationDate time.Time                    `json:"notification_date"`
	Campaigns        []NotificationDigestCampaign `json:"campaigns"`
}

func CampaignStatusFor(now time.Time, startDate time.Time, closedAt *time.Time) string {
	if closedAt != nil {
		return CampaignStatusClosed
	}

	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDay := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	if startDay.After(nowDate) {
		return CampaignStatusScheduled
	}

	return CampaignStatusActive
}

func FormatViewsTarget(value int64) string {
	switch value {
	case 250000:
		return "250K"
	case 500000:
		return "500K"
	case 750000:
		return "750K"
	case 1000000:
		return "1 МЛН"
	case 1500000:
		return "1.5 МЛН"
	case 2000000:
		return "2 МЛН"
	case 3000000:
		return "3 МЛН"
	default:
		return FormatViewsNumber(value)
	}
}

func FormatViewsNumber(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}

	raw := strconv.FormatInt(value, 10)
	if len(raw) <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}

	parts := make([]string, 0, (len(raw)+2)/3)
	for len(raw) > 3 {
		parts = append([]string{raw[len(raw)-3:]}, parts...)
		raw = raw[:len(raw)-3]
	}
	parts = append([]string{raw}, parts...)
	result := strings.Join(parts, " ")
	if negative {
		return "-" + result
	}
	return result
}

func EstimateCloseInDays(remaining int64, dailyGrowth int64) *int64 {
	if remaining <= 0 || dailyGrowth <= 0 {
		return nil
	}

	value := int64(math.Ceil(float64(remaining) / float64(dailyGrowth)))
	return &value
}

func ParseNotificationTime(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("notification time must be HH:MM")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("notification hour must be between 0 and 23")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("notification minute must be between 0 and 59")
	}

	return hour, minute, nil
}
