package campaign_postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) AddUserChannel(ctx context.Context, userID int64, channelID string, channelTitle string) (domain.UserChannel, error) {
	var row domain.UserChannel
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO telegram_user_channels (
			telegram_user_id,
			channel_id,
			channel_title,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, telegram_user_id, channel_id, channel_title, created_at, updated_at
	`, userID, strings.TrimSpace(channelID), strings.TrimSpace(channelTitle)).Scan(
		&row.ID,
		&row.TelegramUserID,
		&row.ChannelID,
		&row.ChannelTitle,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "uq_telegram_user_channel") {
			return domain.UserChannel{}, fmt.Errorf("%w: channel is already linked", core_errors.ErrConflict)
		}
		return domain.UserChannel{}, fmt.Errorf("insert user channel: %w", err)
	}
	return row, nil
}

func (s *Store) GetUserChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	var row domain.UserChannel
	err := s.db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, channel_id, channel_title, created_at, updated_at
		FROM telegram_user_channels
		WHERE telegram_user_id = $1 AND channel_id = $2
	`, userID, strings.TrimSpace(channelID)).Scan(
		&row.ID,
		&row.TelegramUserID,
		&row.ChannelID,
		&row.ChannelTitle,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.UserChannel{}, fmt.Errorf("%w: channel not found", core_errors.ErrNotFound)
		}
		return domain.UserChannel{}, fmt.Errorf("select user channel: %w", err)
	}
	return row, nil
}

func (s *Store) ListUserChannels(ctx context.Context, userID int64) ([]domain.UserChannel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, telegram_user_id, channel_id, channel_title, created_at, updated_at
		FROM telegram_user_channels
		WHERE telegram_user_id = $1
		ORDER BY created_at ASC, id ASC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("select user channels: %w", err)
	}
	defer rows.Close()

	channels := make([]domain.UserChannel, 0)
	for rows.Next() {
		var row domain.UserChannel
		if err := rows.Scan(&row.ID, &row.TelegramUserID, &row.ChannelID, &row.ChannelTitle, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user channel: %w", err)
		}
		channels = append(channels, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user channels: %w", err)
	}

	return channels, nil
}

func (s *Store) DeleteUserChannel(ctx context.Context, userID int64, channelID string) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM telegram_user_channels
		WHERE telegram_user_id = $1 AND channel_id = $2
	`, userID, strings.TrimSpace(channelID))
	if err != nil {
		return fmt.Errorf("delete user channel: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user channel rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: channel not found", core_errors.ErrNotFound)
	}
	return nil
}

func (s *Store) GetUserSettings(ctx context.Context, userID int64) (domain.UserSettings, error) {
	var row domain.UserSettings
	var lastSent sql.NullTime
	var googleEmail sql.NullString
	var googleRefreshToken sql.NullString
	var googleConnectedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT telegram_user_id, notifications_enabled, notification_time, timezone, google_email, google_refresh_token, google_connected_at, last_notification_sent_at, created_at, updated_at
		FROM telegram_user_settings
		WHERE telegram_user_id = $1
	`, userID).Scan(
		&row.TelegramUserID,
		&row.NotificationsEnabled,
		&row.NotificationTime,
		&row.Timezone,
		&googleEmail,
		&googleRefreshToken,
		&googleConnectedAt,
		&lastSent,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.DefaultUserSettings(userID), nil
		}
		return domain.UserSettings{}, fmt.Errorf("select user settings: %w", err)
	}
	if lastSent.Valid {
		row.LastNotificationSentAt = &lastSent.Time
	}
	if googleEmail.Valid {
		row.GoogleEmail = googleEmail.String
	}
	if googleRefreshToken.Valid {
		row.GoogleRefreshToken = googleRefreshToken.String
	}
	if googleConnectedAt.Valid {
		row.GoogleConnectedAt = &googleConnectedAt.Time
	}
	return row, nil
}

func (s *Store) UpsertUserSettings(ctx context.Context, settings domain.UserSettings) (domain.UserSettings, error) {
	if strings.TrimSpace(settings.NotificationTime) == "" {
		settings.NotificationTime = domain.DefaultNotificationTime
	}
	if strings.TrimSpace(settings.Timezone) == "" {
		settings.Timezone = domain.DefaultUserTimezone
	}

	var row domain.UserSettings
	var lastSent sql.NullTime
	var googleEmail sql.NullString
	var googleRefreshToken sql.NullString
	var googleConnectedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO telegram_user_settings (
			telegram_user_id,
			notifications_enabled,
			notification_time,
			timezone,
			google_email,
			google_refresh_token,
			google_connected_at,
			last_notification_sent_at,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, NULL, NULL, NULL, $5, NOW(), NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			notifications_enabled = EXCLUDED.notifications_enabled,
			notification_time = EXCLUDED.notification_time,
			timezone = EXCLUDED.timezone,
			last_notification_sent_at = COALESCE(EXCLUDED.last_notification_sent_at, telegram_user_settings.last_notification_sent_at),
			updated_at = NOW()
		RETURNING telegram_user_id, notifications_enabled, notification_time, timezone, google_email, google_refresh_token, google_connected_at, last_notification_sent_at, created_at, updated_at
	`, settings.TelegramUserID, settings.NotificationsEnabled, settings.NotificationTime, settings.Timezone, nullableTime(settings.LastNotificationSentAt)).Scan(
		&row.TelegramUserID,
		&row.NotificationsEnabled,
		&row.NotificationTime,
		&row.Timezone,
		&googleEmail,
		&googleRefreshToken,
		&googleConnectedAt,
		&lastSent,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("upsert user settings: %w", err)
	}
	if lastSent.Valid {
		row.LastNotificationSentAt = &lastSent.Time
	}
	if googleEmail.Valid {
		row.GoogleEmail = googleEmail.String
	}
	if googleRefreshToken.Valid {
		row.GoogleRefreshToken = googleRefreshToken.String
	}
	if googleConnectedAt.Valid {
		row.GoogleConnectedAt = &googleConnectedAt.Time
	}
	return row, nil
}

func (s *Store) SaveGoogleLink(ctx context.Context, userID int64, email string, refreshToken string, connectedAt time.Time) (domain.UserSettings, error) {
	var row domain.UserSettings
	var lastSent sql.NullTime
	var googleEmail sql.NullString
	var googleRefreshToken sql.NullString
	var googleConnected sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO telegram_user_settings (
			telegram_user_id,
			notifications_enabled,
			notification_time,
			timezone,
			google_email,
			google_refresh_token,
			google_connected_at,
			created_at,
			updated_at
		) VALUES ($1, TRUE, $2, $3, $4, $5, $6, NOW(), NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			google_email = EXCLUDED.google_email,
			google_refresh_token = EXCLUDED.google_refresh_token,
			google_connected_at = EXCLUDED.google_connected_at,
			updated_at = NOW()
		RETURNING telegram_user_id, notifications_enabled, notification_time, timezone, google_email, google_refresh_token, google_connected_at, last_notification_sent_at, created_at, updated_at
	`, userID, domain.DefaultNotificationTime, domain.DefaultUserTimezone, strings.TrimSpace(email), strings.TrimSpace(refreshToken), connectedAt.UTC()).Scan(
		&row.TelegramUserID,
		&row.NotificationsEnabled,
		&row.NotificationTime,
		&row.Timezone,
		&googleEmail,
		&googleRefreshToken,
		&googleConnected,
		&lastSent,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("save google link: %w", err)
	}
	if googleEmail.Valid {
		row.GoogleEmail = googleEmail.String
	}
	if googleRefreshToken.Valid {
		row.GoogleRefreshToken = googleRefreshToken.String
	}
	if googleConnected.Valid {
		row.GoogleConnectedAt = &googleConnected.Time
	}
	if lastSent.Valid {
		row.LastNotificationSentAt = &lastSent.Time
	}
	return row, nil
}

func (s *Store) MarkNotificationSent(ctx context.Context, userID int64, sentAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO telegram_user_settings (telegram_user_id, notifications_enabled, notification_time, timezone, last_notification_sent_at, created_at, updated_at)
		VALUES ($1, TRUE, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			last_notification_sent_at = EXCLUDED.last_notification_sent_at,
			updated_at = NOW()
	`, userID, domain.DefaultNotificationTime, domain.DefaultUserTimezone, sentAt.UTC())
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (s *Store) ListNotificationUsers(ctx context.Context) ([]domain.UserSettings, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT users.telegram_user_id,
			COALESCE(settings.notifications_enabled, TRUE) AS notifications_enabled,
			COALESCE(settings.notification_time, $1) AS notification_time,
			COALESCE(settings.timezone, $2) AS timezone,
			settings.last_notification_sent_at,
			COALESCE(settings.created_at, NOW()) AS created_at,
			COALESCE(settings.updated_at, NOW()) AS updated_at
		FROM (
			SELECT telegram_user_id FROM telegram_user_settings
			UNION
			SELECT DISTINCT telegram_user_id FROM ad_campaigns WHERE closed_at IS NULL
		) AS users
		LEFT JOIN telegram_user_settings AS settings ON settings.telegram_user_id = users.telegram_user_id
		WHERE COALESCE(settings.notifications_enabled, TRUE) = TRUE
		ORDER BY users.telegram_user_id ASC
	`, domain.DefaultNotificationTime, domain.DefaultUserTimezone)
	if err != nil {
		return nil, fmt.Errorf("select notification users: %w", err)
	}
	defer rows.Close()

	result := make([]domain.UserSettings, 0)
	for rows.Next() {
		var row domain.UserSettings
		var lastSent sql.NullTime
		if err := rows.Scan(&row.TelegramUserID, &row.NotificationsEnabled, &row.NotificationTime, &row.Timezone, &lastSent, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan notification user: %w", err)
		}
		if lastSent.Valid {
			row.LastNotificationSentAt = &lastSent.Time
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification users: %w", err)
	}
	return result, nil
}

func (s *Store) CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	var row domain.Campaign
	var targetViews sql.NullInt64
	var closedAt sql.NullTime
	var closeReason sql.NullString
	var lastSnapshotAt sql.NullTime
	var lastDailyGrowth sql.NullInt64
	var estimatedCloseDate sql.NullTime
	var spreadsheetID sql.NullString
	var spreadsheetURL sql.NullString
	columnsPayload, err := json.Marshal(domain.NormalizeStatsColumns(campaign.Columns))
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("marshal campaign columns: %w", err)
	}
	if campaign.TargetViews != nil {
		targetViews = sql.NullInt64{Int64: *campaign.TargetViews, Valid: true}
	}

	err = s.db.QueryRowContext(ctx, `
		INSERT INTO ad_campaigns (
			telegram_user_id,
			channel_id,
			channel_title,
			keyword,
			start_date,
			timezone,
			target_views,
			columns,
			status,
			export_jwt,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, NOW(), NOW())
		RETURNING id, telegram_user_id, channel_id, channel_title, keyword, start_date, timezone, target_views, columns, status, export_jwt,
			closed_at, close_reason, last_snapshot_at, last_total_views, last_daily_growth, estimated_close_date, spreadsheet_id, spreadsheet_url, created_at, updated_at
	`, campaign.TelegramUserID, campaign.ChannelID, campaign.ChannelTitle, campaign.Keyword, campaign.StartDate.UTC(), campaign.Timezone, targetViews, string(columnsPayload), campaign.Status, campaign.ExportJWT).Scan(
		&row.ID,
		&row.TelegramUserID,
		&row.ChannelID,
		&row.ChannelTitle,
		&row.Keyword,
		&row.StartDate,
		&row.Timezone,
		&targetViews,
		&columnsPayload,
		&row.Status,
		&row.ExportJWT,
		&closedAt,
		&closeReason,
		&lastSnapshotAt,
		&row.LastTotalViews,
		&lastDailyGrowth,
		&estimatedCloseDate,
		&spreadsheetID,
		&spreadsheetURL,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "uq_ad_campaign_identity") {
			return domain.Campaign{}, fmt.Errorf("%w: campaign already exists", core_errors.ErrConflict)
		}
		if strings.Contains(message, "uq_ad_campaign_export_jwt") {
			return domain.Campaign{}, fmt.Errorf("%w: campaign export token already exists", core_errors.ErrConflict)
		}
		return domain.Campaign{}, fmt.Errorf("insert campaign: %w", err)
	}
	if targetViews.Valid {
		row.TargetViews = &targetViews.Int64
	}
	row.Columns = parseCampaignColumns(columnsPayload)
	if closedAt.Valid {
		row.ClosedAt = &closedAt.Time
	}
	if closeReason.Valid {
		row.CloseReason = closeReason.String
	}
	if lastSnapshotAt.Valid {
		row.LastSnapshotAt = &lastSnapshotAt.Time
	}
	if lastDailyGrowth.Valid {
		row.LastDailyGrowth = &lastDailyGrowth.Int64
	}
	if estimatedCloseDate.Valid {
		row.EstimatedCloseDate = &estimatedCloseDate.Time
	}
	if spreadsheetID.Valid {
		row.SpreadsheetID = spreadsheetID.String
	}
	if spreadsheetURL.Valid {
		row.SpreadsheetURL = spreadsheetURL.String
	}
	return row, nil
}

func (s *Store) GetCampaignByID(ctx context.Context, campaignID int64) (domain.Campaign, error) {
	return s.getCampaign(ctx, `SELECT id, telegram_user_id, channel_id, channel_title, keyword, start_date, timezone, target_views, columns, status, export_jwt,
		closed_at, close_reason, last_snapshot_at, last_total_views, last_daily_growth, estimated_close_date, spreadsheet_id, spreadsheet_url, created_at, updated_at
		FROM ad_campaigns WHERE id = $1`, campaignID)
}

func (s *Store) GetCampaignByIDForUser(ctx context.Context, campaignID int64, userID int64) (domain.Campaign, error) {
	return s.getCampaign(ctx, `SELECT id, telegram_user_id, channel_id, channel_title, keyword, start_date, timezone, target_views, columns, status, export_jwt,
		closed_at, close_reason, last_snapshot_at, last_total_views, last_daily_growth, estimated_close_date, spreadsheet_id, spreadsheet_url, created_at, updated_at
		FROM ad_campaigns WHERE id = $1 AND telegram_user_id = $2`, campaignID, userID)
}

func (s *Store) GetCampaignByExportJWT(ctx context.Context, token string) (domain.Campaign, error) {
	return s.getCampaign(ctx, `SELECT id, telegram_user_id, channel_id, channel_title, keyword, start_date, timezone, target_views, columns, status, export_jwt,
		closed_at, close_reason, last_snapshot_at, last_total_views, last_daily_growth, estimated_close_date, spreadsheet_id, spreadsheet_url, created_at, updated_at
		FROM ad_campaigns WHERE export_jwt = $1`, token)
}

func (s *Store) ListUserCampaigns(ctx context.Context, userID int64) ([]domain.Campaign, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, telegram_user_id, channel_id, channel_title, keyword, start_date, timezone, target_views, columns, status, export_jwt,
			closed_at, close_reason, last_snapshot_at, last_total_views, last_daily_growth, estimated_close_date, spreadsheet_id, spreadsheet_url, created_at, updated_at
		FROM ad_campaigns
		WHERE telegram_user_id = $1
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("select user campaigns: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Campaign, 0)
	for rows.Next() {
		item, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user campaigns: %w", err)
	}
	return items, nil
}

func (s *Store) SaveCampaignSnapshot(ctx context.Context, campaign domain.Campaign, snapshot domain.CampaignSnapshot) (domain.CampaignSnapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("begin snapshot tx: %w", err)
	}

	var snapshotID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO ad_campaign_snapshots (
			campaign_id,
			snapshot_at,
			total_views,
			remaining_views,
			daily_growth,
			estimated_close_date,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id
	`, campaign.ID, snapshot.SnapshotAt.UTC(), snapshot.TotalViews, nullableInt64(snapshot.RemainingViews), nullableInt64(snapshot.DailyGrowth), nullableDate(snapshot.EstimatedCloseDate)).Scan(&snapshotID)
	if err != nil {
		_ = tx.Rollback()
		return domain.CampaignSnapshot{}, fmt.Errorf("insert campaign snapshot: %w", err)
	}

	for idx, video := range snapshot.Videos {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO ad_campaign_snapshot_videos (
				snapshot_id,
				position,
				video_id,
				name,
				publish_date,
				views,
				url,
				views_updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, snapshotID, idx, video.VideoID, video.Name, video.PublishDate.UTC(), int64(video.Views), video.URL, video.ViewsUpdatedAt.UTC())
		if err != nil {
			_ = tx.Rollback()
			return domain.CampaignSnapshot{}, fmt.Errorf("insert snapshot video: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE ad_campaigns
		SET status = $2,
			closed_at = $3,
			close_reason = $4,
			last_snapshot_at = $5,
			last_total_views = $6,
			last_daily_growth = $7,
			estimated_close_date = $8,
			updated_at = NOW()
		WHERE id = $1
	`, campaign.ID, campaign.Status, nullableTime(campaign.ClosedAt), nullableString(campaign.CloseReason), snapshot.SnapshotAt.UTC(), snapshot.TotalViews, nullableInt64(snapshot.DailyGrowth), nullableDate(snapshot.EstimatedCloseDate))
	if err != nil {
		_ = tx.Rollback()
		return domain.CampaignSnapshot{}, fmt.Errorf("update campaign after snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("commit snapshot tx: %w", err)
	}

	snapshot.ID = snapshotID
	return snapshot, nil
}

func (s *Store) CloseCampaign(ctx context.Context, campaignID int64, closedAt time.Time, reason string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE ad_campaigns
		SET status = $2,
			closed_at = $3,
			close_reason = $4,
			updated_at = NOW()
		WHERE id = $1 AND closed_at IS NULL
	`, campaignID, domain.CampaignStatusClosed, closedAt.UTC(), strings.TrimSpace(reason))
	if err != nil {
		return fmt.Errorf("close campaign: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("close campaign rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: campaign not found or already closed", core_errors.ErrNotFound)
	}
	return nil
}

func (s *Store) UpdateCampaignSpreadsheet(ctx context.Context, campaignID int64, spreadsheetID string, spreadsheetURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE ad_campaigns
		SET spreadsheet_id = $2,
			spreadsheet_url = $3,
			updated_at = NOW()
		WHERE id = $1
	`, campaignID, nullableString(spreadsheetID), nullableString(spreadsheetURL))
	if err != nil {
		return fmt.Errorf("update campaign spreadsheet: %w", err)
	}
	return nil
}

func (s *Store) GetLatestSnapshot(ctx context.Context, campaignID int64) (domain.CampaignSnapshot, error) {
	var row domain.CampaignSnapshot
	var remaining sql.NullInt64
	var dailyGrowth sql.NullInt64
	var estimated sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, campaign_id, snapshot_at, total_views, remaining_views, daily_growth, estimated_close_date, created_at
		FROM ad_campaign_snapshots
		WHERE campaign_id = $1
		ORDER BY snapshot_at DESC, id DESC
		LIMIT 1
	`, campaignID).Scan(&row.ID, &row.CampaignID, &row.SnapshotAt, &row.TotalViews, &remaining, &dailyGrowth, &estimated, &row.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignSnapshot{}, fmt.Errorf("%w: snapshot not found", core_errors.ErrNotFound)
		}
		return domain.CampaignSnapshot{}, fmt.Errorf("select latest snapshot: %w", err)
	}
	if remaining.Valid {
		row.RemainingViews = &remaining.Int64
	}
	if dailyGrowth.Valid {
		row.DailyGrowth = &dailyGrowth.Int64
	}
	if estimated.Valid {
		row.EstimatedCloseDate = &estimated.Time
	}
	return row, nil
}

func (s *Store) GetLatestSnapshotWithVideos(ctx context.Context, campaignID int64) (domain.CampaignSnapshot, error) {
	snapshot, err := s.GetLatestSnapshot(ctx, campaignID)
	if err != nil {
		return domain.CampaignSnapshot{}, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT video_id, name, publish_date, views, url, views_updated_at
		FROM ad_campaign_snapshot_videos
		WHERE snapshot_id = $1
		ORDER BY position ASC
	`, snapshot.ID)
	if err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("select snapshot videos: %w", err)
	}
	defer rows.Close()

	videos := make([]domain.StatsVideo, 0)
	for rows.Next() {
		var video domain.StatsVideo
		var views int64
		if err := rows.Scan(&video.VideoID, &video.Name, &video.PublishDate, &views, &video.URL, &video.ViewsUpdatedAt); err != nil {
			return domain.CampaignSnapshot{}, fmt.Errorf("scan snapshot video: %w", err)
		}
		video.Views = uint64(views)
		videos = append(videos, video)
	}
	if err := rows.Err(); err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("iterate snapshot videos: %w", err)
	}
	snapshot.Videos = videos
	return snapshot, nil
}

func (s *Store) GetPreviousSnapshot(ctx context.Context, campaignID int64, before time.Time) (domain.CampaignSnapshot, error) {
	var row domain.CampaignSnapshot
	var remaining sql.NullInt64
	var dailyGrowth sql.NullInt64
	var estimated sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, campaign_id, snapshot_at, total_views, remaining_views, daily_growth, estimated_close_date, created_at
		FROM ad_campaign_snapshots
		WHERE campaign_id = $1 AND snapshot_at < $2
		ORDER BY snapshot_at DESC, id DESC
		LIMIT 1
	`, campaignID, before.UTC()).Scan(&row.ID, &row.CampaignID, &row.SnapshotAt, &row.TotalViews, &remaining, &dailyGrowth, &estimated, &row.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignSnapshot{}, fmt.Errorf("%w: snapshot not found", core_errors.ErrNotFound)
		}
		return domain.CampaignSnapshot{}, fmt.Errorf("select previous snapshot: %w", err)
	}
	if remaining.Valid {
		row.RemainingViews = &remaining.Int64
	}
	if dailyGrowth.Valid {
		row.DailyGrowth = &dailyGrowth.Int64
	}
	if estimated.Valid {
		row.EstimatedCloseDate = &estimated.Time
	}
	return row, nil
}

func (s *Store) GetInputSession(ctx context.Context, userID int64) (domain.CampaignInputSession, error) {
	var row domain.CampaignInputSession
	var payload []byte
	var expires sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT telegram_user_id, flow, step, payload, expires_at, created_at, updated_at
		FROM telegram_input_sessions
		WHERE telegram_user_id = $1
	`, userID).Scan(&row.TelegramUserID, &row.Flow, &row.Step, &payload, &expires, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignInputSession{}, fmt.Errorf("%w: input session not found", core_errors.ErrNotFound)
		}
		return domain.CampaignInputSession{}, fmt.Errorf("select input session: %w", err)
	}
	row.Payload = string(payload)
	if expires.Valid {
		row.ExpiresAt = &expires.Time
	}
	return row, nil
}

func (s *Store) UpsertInputSession(ctx context.Context, session domain.CampaignInputSession) (domain.CampaignInputSession, error) {
	payload := strings.TrimSpace(session.Payload)
	if payload == "" {
		payload = "{}"
	}
	if !json.Valid([]byte(payload)) {
		return domain.CampaignInputSession{}, fmt.Errorf("%w: input session payload must be valid json", core_errors.ErrInvalidArgument)
	}

	var row domain.CampaignInputSession
	var stored []byte
	var expires sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO telegram_input_sessions (
			telegram_user_id,
			flow,
			step,
			payload,
			expires_at,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4::jsonb, $5, NOW(), NOW())
		ON CONFLICT (telegram_user_id) DO UPDATE SET
			flow = EXCLUDED.flow,
			step = EXCLUDED.step,
			payload = EXCLUDED.payload,
			expires_at = EXCLUDED.expires_at,
			updated_at = NOW()
		RETURNING telegram_user_id, flow, step, payload, expires_at, created_at, updated_at
	`, session.TelegramUserID, session.Flow, session.Step, payload, nullableTime(session.ExpiresAt)).Scan(
		&row.TelegramUserID,
		&row.Flow,
		&row.Step,
		&stored,
		&expires,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return domain.CampaignInputSession{}, fmt.Errorf("upsert input session: %w", err)
	}
	row.Payload = string(stored)
	if expires.Valid {
		row.ExpiresAt = &expires.Time
	}
	return row, nil
}

func (s *Store) DeleteInputSession(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM telegram_input_sessions WHERE telegram_user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete input session: %w", err)
	}
	return nil
}

func (s *Store) CreatePublicSession(ctx context.Context, session domain.PublicUserSession) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO public_user_sessions (
			id,
			telegram_user_id,
			refresh_token_hash,
			expires_at,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW())
	`, session.ID, session.TelegramUserID, session.RefreshTokenHash, session.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("insert public session: %w", err)
	}
	return nil
}

func (s *Store) GetPublicSession(ctx context.Context, sessionID string) (domain.PublicUserSession, error) {
	var row domain.PublicUserSession
	var revoked sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, refresh_token_hash, expires_at, revoked_at, created_at, updated_at
		FROM public_user_sessions
		WHERE id = $1
	`, strings.TrimSpace(sessionID)).Scan(&row.ID, &row.TelegramUserID, &row.RefreshTokenHash, &row.ExpiresAt, &revoked, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.PublicUserSession{}, fmt.Errorf("%w: public session not found", core_errors.ErrNotFound)
		}
		return domain.PublicUserSession{}, fmt.Errorf("select public session: %w", err)
	}
	if revoked.Valid {
		row.RevokedAt = &revoked.Time
	}
	return row, nil
}

func (s *Store) UpdatePublicSession(ctx context.Context, sessionID string, refreshTokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE public_user_sessions
		SET refresh_token_hash = $2,
			expires_at = $3,
			revoked_at = NULL,
			updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(sessionID), refreshTokenHash, expiresAt.UTC())
	if err != nil {
		return fmt.Errorf("update public session: %w", err)
	}
	return nil
}

func (s *Store) RevokePublicSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE public_user_sessions
		SET revoked_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return fmt.Errorf("revoke public session: %w", err)
	}
	return nil
}

func (s *Store) getCampaign(ctx context.Context, query string, args ...any) (domain.Campaign, error) {
	row, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("query campaign: %w", err)
	}
	defer row.Close()
	if !row.Next() {
		if err := row.Err(); err != nil {
			return domain.Campaign{}, fmt.Errorf("iterate campaign row: %w", err)
		}
		return domain.Campaign{}, fmt.Errorf("%w: campaign not found", core_errors.ErrNotFound)
	}
	return scanCampaign(row)
}

type campaignScanner interface {
	Scan(dest ...any) error
}

func scanCampaign(scanner campaignScanner) (domain.Campaign, error) {
	var row domain.Campaign
	var targetViews sql.NullInt64
	var columnsPayload []byte
	var closedAt sql.NullTime
	var lastSnapshotAt sql.NullTime
	var lastDailyGrowth sql.NullInt64
	var estimatedCloseDate sql.NullTime
	var closeReason sql.NullString
	var spreadsheetID sql.NullString
	var spreadsheetURL sql.NullString

	err := scanner.Scan(
		&row.ID,
		&row.TelegramUserID,
		&row.ChannelID,
		&row.ChannelTitle,
		&row.Keyword,
		&row.StartDate,
		&row.Timezone,
		&targetViews,
		&columnsPayload,
		&row.Status,
		&row.ExportJWT,
		&closedAt,
		&closeReason,
		&lastSnapshotAt,
		&row.LastTotalViews,
		&lastDailyGrowth,
		&estimatedCloseDate,
		&spreadsheetID,
		&spreadsheetURL,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("scan campaign: %w", err)
	}

	if targetViews.Valid {
		row.TargetViews = &targetViews.Int64
	}
	row.Columns = parseCampaignColumns(columnsPayload)
	if closedAt.Valid {
		row.ClosedAt = &closedAt.Time
	}
	if closeReason.Valid {
		row.CloseReason = closeReason.String
	}
	if lastSnapshotAt.Valid {
		row.LastSnapshotAt = &lastSnapshotAt.Time
	}
	if lastDailyGrowth.Valid {
		row.LastDailyGrowth = &lastDailyGrowth.Int64
	}
	if estimatedCloseDate.Valid {
		row.EstimatedCloseDate = &estimatedCloseDate.Time
	}
	if spreadsheetID.Valid {
		row.SpreadsheetID = spreadsheetID.String
	}
	if spreadsheetURL.Valid {
		row.SpreadsheetURL = spreadsheetURL.String
	}
	return row, nil
}

func parseCampaignColumns(payload []byte) []domain.StatsColumn {
	if len(payload) == 0 {
		return domain.DefaultStatsColumns()
	}
	var columns []domain.StatsColumn
	if err := json.Unmarshal(payload, &columns); err != nil {
		return domain.DefaultStatsColumns()
	}
	return domain.NormalizeStatsColumns(columns)
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
