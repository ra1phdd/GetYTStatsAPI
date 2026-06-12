package campaign_postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	postgres_sqlc "getytstatsapi/internal/core/infra/postgres/sqlc"
)

type Store struct {
	db      *sql.DB
	queries *postgres_sqlc.Queries
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db, queries: postgres_sqlc.New(db)}
}

func (s *Store) AddUserChannel(ctx context.Context, userID int64, channelID string, channelTitle string) (domain.UserChannel, error) {
	row, err := s.queries.AddUserChannel(ctx, postgres_sqlc.AddUserChannelParams{
		TelegramUserID: userID,
		ChannelID:      strings.TrimSpace(channelID),
		ChannelTitle:   strings.TrimSpace(channelTitle),
	})
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "uq_telegram_user_channel") {
			return domain.UserChannel{}, fmt.Errorf("%w: channel is already linked", core_errors.ErrConflict)
		}
		return domain.UserChannel{}, fmt.Errorf("insert user channel: %w", err)
	}
	return userChannelFromRow(row), nil
}

func (s *Store) GetUserChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	row, err := s.queries.GetUserChannel(ctx, postgres_sqlc.GetUserChannelParams{
		TelegramUserID: userID,
		ChannelID:      strings.TrimSpace(channelID),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.UserChannel{}, fmt.Errorf("%w: channel not found", core_errors.ErrNotFound)
		}
		return domain.UserChannel{}, fmt.Errorf("select user channel: %w", err)
	}
	return userChannelFromRow(row), nil
}

func (s *Store) ListUserChannels(ctx context.Context, userID int64) ([]domain.UserChannel, error) {
	rows, err := s.queries.ListUserChannels(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select user channels: %w", err)
	}

	channels := make([]domain.UserChannel, 0, len(rows))
	for _, row := range rows {
		channels = append(channels, userChannelFromRow(row))
	}

	return channels, nil
}

func (s *Store) DeleteUserChannel(ctx context.Context, userID int64, channelID string) error {
	affected, err := s.queries.DeleteUserChannel(ctx, postgres_sqlc.DeleteUserChannelParams{
		TelegramUserID: userID,
		ChannelID:      strings.TrimSpace(channelID),
	})
	if err != nil {
		return fmt.Errorf("delete user channel: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: channel not found", core_errors.ErrNotFound)
	}
	return nil
}

func (s *Store) GetUserSettings(ctx context.Context, userID int64) (domain.UserSettings, error) {
	row, err := s.queries.GetUserSettings(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.DefaultUserSettings(userID), nil
		}
		return domain.UserSettings{}, fmt.Errorf("select user settings: %w", err)
	}
	return userSettingsFromRow(row), nil
}

func (s *Store) UpsertUserSettings(ctx context.Context, settings domain.UserSettings) (domain.UserSettings, error) {
	if strings.TrimSpace(settings.NotificationTime) == "" {
		settings.NotificationTime = domain.DefaultNotificationTime
	}
	settings.NotificationIntervalMinutes = domain.NormalizeNotificationIntervalMinutes(settings.NotificationIntervalMinutes)
	if strings.TrimSpace(settings.Timezone) == "" {
		settings.Timezone = domain.DefaultUserTimezone
	}

	row, err := s.queries.UpsertUserSettings(ctx, postgres_sqlc.UpsertUserSettingsParams{
		TelegramUserID:              settings.TelegramUserID,
		NotificationsEnabled:        settings.NotificationsEnabled,
		NotificationTime:            settings.NotificationTime,
		NotificationIntervalMinutes: int32(settings.NotificationIntervalMinutes),
		Timezone:                    settings.Timezone,
		LastNotificationSentAt:      nullableTime(settings.LastNotificationSentAt),
	})
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("upsert user settings: %w", err)
	}
	return userSettingsFromRow(row), nil
}

func (s *Store) SaveGoogleLink(ctx context.Context, userID int64, email string, refreshToken string, connectedAt time.Time) (domain.UserSettings, error) {
	row, err := s.queries.SaveGoogleLink(ctx, postgres_sqlc.SaveGoogleLinkParams{
		TelegramUserID:              userID,
		NotificationTime:            domain.DefaultNotificationTime,
		NotificationIntervalMinutes: int32(domain.DefaultNotificationInterval / time.Minute),
		Timezone:                    domain.DefaultUserTimezone,
		GoogleEmail:                 nullableString(strings.TrimSpace(email)),
		GoogleRefreshToken:          nullableString(strings.TrimSpace(refreshToken)),
		GoogleConnectedAt:           nullableTime(&connectedAt),
	})
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("save google link: %w", err)
	}
	return userSettingsFromRow(row), nil
}

func (s *Store) MarkNotificationSent(ctx context.Context, userID int64, sentAt time.Time) error {
	err := s.queries.MarkNotificationSent(ctx, postgres_sqlc.MarkNotificationSentParams{
		TelegramUserID:              userID,
		NotificationTime:            domain.DefaultNotificationTime,
		NotificationIntervalMinutes: int32(domain.DefaultNotificationInterval / time.Minute),
		Timezone:                    domain.DefaultUserTimezone,
		LastNotificationSentAt:      nullableTime(&sentAt),
	})
	if err != nil {
		return fmt.Errorf("mark notification sent: %w", err)
	}
	return nil
}

func (s *Store) ListNotificationUsers(ctx context.Context) ([]domain.UserSettings, error) {
	rows, err := s.queries.ListNotificationUsers(ctx, postgres_sqlc.ListNotificationUsersParams{
		NotificationTime:            domain.DefaultNotificationTime,
		NotificationIntervalMinutes: int32(domain.DefaultNotificationInterval / time.Minute),
		Timezone:                    domain.DefaultUserTimezone,
	})
	if err != nil {
		return nil, fmt.Errorf("select notification users: %w", err)
	}

	result := make([]domain.UserSettings, 0, len(rows))
	for _, row := range rows {
		result = append(result, notificationUserFromRow(row))
	}
	return result, nil
}

func (s *Store) CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	columnsPayload, err := json.Marshal(domain.NormalizeStatsColumns(campaign.Columns))
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("marshal campaign columns: %w", err)
	}
	if _, err := s.getCampaignByChannelIdentity(ctx, campaign.ChannelID, campaign.Keyword, campaign.StartDate); err == nil {
		return domain.Campaign{}, fmt.Errorf("%w: campaign already exists", core_errors.ErrConflict)
	} else if err != nil && !errors.Is(err, core_errors.ErrNotFound) {
		return domain.Campaign{}, err
	}

	row, err := s.queries.CreateCampaign(ctx, postgres_sqlc.CreateCampaignParams{
		TelegramUserID: campaign.TelegramUserID,
		ChannelID:      campaign.ChannelID,
		ChannelTitle:   campaign.ChannelTitle,
		Keyword:        campaign.Keyword,
		StartDate:      campaign.StartDate.UTC(),
		Timezone:       campaign.Timezone,
		TargetViews:    nullableInt64(campaign.TargetViews),
		Column8:        columnsPayload,
		Status:         campaign.Status,
		ExportJwt:      campaign.ExportJWT,
	})
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
	return campaignFromRow(row), nil
}

func (s *Store) GetCampaignByID(ctx context.Context, campaignID int64) (domain.Campaign, error) {
	row, err := s.queries.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Campaign{}, fmt.Errorf("%w: campaign not found", core_errors.ErrNotFound)
		}
		return domain.Campaign{}, fmt.Errorf("select campaign: %w", err)
	}
	return campaignFromRow(row), nil
}

func (s *Store) GetCampaignByIDForUser(ctx context.Context, campaignID int64, userID int64) (domain.Campaign, error) {
	row, err := s.queries.GetCampaignByIDForUser(ctx, postgres_sqlc.GetCampaignByIDForUserParams{
		ID:             campaignID,
		TelegramUserID: userID,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Campaign{}, fmt.Errorf("%w: campaign not found", core_errors.ErrNotFound)
		}
		return domain.Campaign{}, fmt.Errorf("select campaign: %w", err)
	}
	return campaignFromRow(row), nil
}

func (s *Store) GetCampaignByExportJWT(ctx context.Context, token string) (domain.Campaign, error) {
	row, err := s.queries.GetCampaignByExportJWT(ctx, token)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Campaign{}, fmt.Errorf("%w: campaign not found", core_errors.ErrNotFound)
		}
		return domain.Campaign{}, fmt.Errorf("select campaign: %w", err)
	}
	return campaignFromRow(row), nil
}

func (s *Store) ListUserCampaigns(ctx context.Context, userID int64) ([]domain.Campaign, error) {
	rows, err := s.queries.ListUserCampaigns(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("select user campaigns: %w", err)
	}

	items := make([]domain.Campaign, 0, len(rows))
	for _, row := range rows {
		items = append(items, campaignFromRow(row))
	}
	return items, nil
}

func (s *Store) getCampaignByChannelIdentity(ctx context.Context, channelID string, keyword string, startDate time.Time) (domain.Campaign, error) {
	row, err := s.queries.GetCampaignByChannelIdentity(ctx, postgres_sqlc.GetCampaignByChannelIdentityParams{
		ChannelID: strings.TrimSpace(channelID),
		Keyword:   strings.TrimSpace(keyword),
		StartDate: time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.Campaign{}, fmt.Errorf("%w: campaign not found", core_errors.ErrNotFound)
		}
		return domain.Campaign{}, fmt.Errorf("select campaign: %w", err)
	}
	return campaignFromRow(row), nil
}

func (s *Store) SaveCampaignSnapshot(ctx context.Context, campaign domain.Campaign, snapshot domain.CampaignSnapshot) (domain.CampaignSnapshot, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("begin snapshot tx: %w", err)
	}

	queries := s.queries.WithTx(tx)
	snapshotID, err := queries.CreateCampaignSnapshot(ctx, postgres_sqlc.CreateCampaignSnapshotParams{
		CampaignID:         campaign.ID,
		SnapshotAt:         snapshot.SnapshotAt.UTC(),
		TotalViews:         snapshot.TotalViews,
		RemainingViews:     nullableInt64(snapshot.RemainingViews),
		DailyGrowth:        nullableInt64(snapshot.DailyGrowth),
		EstimatedCloseDate: nullableDate(snapshot.EstimatedCloseDate),
	})
	if err != nil {
		_ = tx.Rollback()
		return domain.CampaignSnapshot{}, fmt.Errorf("insert campaign snapshot: %w", err)
	}

	for idx, video := range snapshot.Videos {
		err := queries.CreateCampaignSnapshotVideo(ctx, postgres_sqlc.CreateCampaignSnapshotVideoParams{
			SnapshotID:     snapshotID,
			Position:       int32(idx),
			VideoID:        video.VideoID,
			Name:           video.Name,
			PublishDate:    video.PublishDate.UTC(),
			Views:          int64(video.Views),
			Url:            video.URL,
			ViewsUpdatedAt: video.ViewsUpdatedAt.UTC(),
		})
		if err != nil {
			_ = tx.Rollback()
			return domain.CampaignSnapshot{}, fmt.Errorf("insert snapshot video: %w", err)
		}
	}

	err = queries.UpdateCampaignAfterSnapshot(ctx, postgres_sqlc.UpdateCampaignAfterSnapshotParams{
		ID:                 campaign.ID,
		Status:             campaign.Status,
		ClosedAt:           nullableTime(campaign.ClosedAt),
		CloseReason:        nullableString(campaign.CloseReason),
		LastSnapshotAt:     nullableTime(&snapshot.SnapshotAt),
		LastTotalViews:     snapshot.TotalViews,
		LastDailyGrowth:    nullableInt64(snapshot.DailyGrowth),
		EstimatedCloseDate: nullableDate(snapshot.EstimatedCloseDate),
	})
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
	affected, err := s.queries.CloseCampaign(ctx, postgres_sqlc.CloseCampaignParams{
		ID:          campaignID,
		Status:      domain.CampaignStatusClosed,
		ClosedAt:    nullableTime(&closedAt),
		CloseReason: nullableString(strings.TrimSpace(reason)),
	})
	if err != nil {
		return fmt.Errorf("close campaign: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: campaign not found or already closed", core_errors.ErrNotFound)
	}
	return nil
}

func (s *Store) UpdateCampaignColumns(ctx context.Context, campaignID int64, columns []domain.StatsColumn) error {
	payload, err := json.Marshal(domain.NormalizeStatsColumns(columns))
	if err != nil {
		return fmt.Errorf("marshal campaign columns: %w", err)
	}
	err = s.queries.UpdateCampaignColumns(ctx, postgres_sqlc.UpdateCampaignColumnsParams{
		ID:      campaignID,
		Column2: payload,
	})
	if err != nil {
		return fmt.Errorf("update campaign columns: %w", err)
	}
	return nil
}

func (s *Store) UpdateCampaignTarget(ctx context.Context, campaignID int64, targetViews *int64, status string, closedAt *time.Time, closeReason string, estimatedCloseDate *time.Time) error {
	err := s.queries.UpdateCampaignTarget(ctx, postgres_sqlc.UpdateCampaignTargetParams{
		ID:                 campaignID,
		TargetViews:        nullableInt64(targetViews),
		Status:             strings.TrimSpace(status),
		ClosedAt:           nullableTime(closedAt),
		CloseReason:        nullableString(closeReason),
		EstimatedCloseDate: nullableDate(estimatedCloseDate),
	})
	if err != nil {
		return fmt.Errorf("update campaign target: %w", err)
	}
	return nil
}

func (s *Store) UpdateCampaignSpreadsheet(ctx context.Context, campaignID int64, spreadsheetID string, spreadsheetURL string) error {
	err := s.queries.UpdateCampaignSpreadsheet(ctx, postgres_sqlc.UpdateCampaignSpreadsheetParams{
		ID:             campaignID,
		SpreadsheetID:  nullableString(spreadsheetID),
		SpreadsheetUrl: nullableString(spreadsheetURL),
	})
	if err != nil {
		return fmt.Errorf("update campaign spreadsheet: %w", err)
	}
	return nil
}

func (s *Store) GetLatestSnapshot(ctx context.Context, campaignID int64) (domain.CampaignSnapshot, error) {
	row, err := s.queries.GetLatestSnapshot(ctx, campaignID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignSnapshot{}, fmt.Errorf("%w: snapshot not found", core_errors.ErrNotFound)
		}
		return domain.CampaignSnapshot{}, fmt.Errorf("select latest snapshot: %w", err)
	}
	return snapshotFromRow(row), nil
}

func (s *Store) GetLatestSnapshotWithVideos(ctx context.Context, campaignID int64) (domain.CampaignSnapshot, error) {
	snapshot, err := s.GetLatestSnapshot(ctx, campaignID)
	if err != nil {
		return domain.CampaignSnapshot{}, err
	}

	rows, err := s.queries.ListCampaignSnapshotVideos(ctx, snapshot.ID)
	if err != nil {
		return domain.CampaignSnapshot{}, fmt.Errorf("select snapshot videos: %w", err)
	}

	videos := make([]domain.StatsVideo, 0, len(rows))
	for _, row := range rows {
		var video domain.StatsVideo
		video.VideoID = row.VideoID
		video.Name = row.Name
		video.PublishDate = row.PublishDate
		video.Views = uint64(row.Views)
		video.URL = row.Url
		video.ViewsUpdatedAt = row.ViewsUpdatedAt
		videos = append(videos, video)
	}
	snapshot.Videos = videos
	return snapshot, nil
}

func (s *Store) GetPreviousSnapshot(ctx context.Context, campaignID int64, before time.Time) (domain.CampaignSnapshot, error) {
	row, err := s.queries.GetPreviousSnapshot(ctx, postgres_sqlc.GetPreviousSnapshotParams{
		CampaignID: campaignID,
		SnapshotAt: before.UTC(),
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignSnapshot{}, fmt.Errorf("%w: snapshot not found", core_errors.ErrNotFound)
		}
		return domain.CampaignSnapshot{}, fmt.Errorf("select previous snapshot: %w", err)
	}
	return snapshotFromRow(row), nil
}

func (s *Store) GetInputSession(ctx context.Context, userID int64) (domain.CampaignInputSession, error) {
	row, err := s.queries.GetInputSession(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.CampaignInputSession{}, fmt.Errorf("%w: input session not found", core_errors.ErrNotFound)
		}
		return domain.CampaignInputSession{}, fmt.Errorf("select input session: %w", err)
	}
	return inputSessionFromRow(row), nil
}

func (s *Store) UpsertInputSession(ctx context.Context, session domain.CampaignInputSession) (domain.CampaignInputSession, error) {
	payload := strings.TrimSpace(session.Payload)
	if payload == "" {
		payload = "{}"
	}
	if !json.Valid([]byte(payload)) {
		return domain.CampaignInputSession{}, fmt.Errorf("%w: input session payload must be valid json", core_errors.ErrInvalidArgument)
	}

	row, err := s.queries.UpsertInputSession(ctx, postgres_sqlc.UpsertInputSessionParams{
		TelegramUserID: session.TelegramUserID,
		Flow:           session.Flow,
		Step:           session.Step,
		Column4:        []byte(payload),
		ExpiresAt:      nullableTime(session.ExpiresAt),
	})
	if err != nil {
		return domain.CampaignInputSession{}, fmt.Errorf("upsert input session: %w", err)
	}
	return inputSessionFromRow(row), nil
}

func (s *Store) DeleteInputSession(ctx context.Context, userID int64) error {
	err := s.queries.DeleteInputSession(ctx, userID)
	if err != nil {
		return fmt.Errorf("delete input session: %w", err)
	}
	return nil
}

func (s *Store) CreatePublicSession(ctx context.Context, session domain.PublicUserSession) error {
	err := s.queries.CreatePublicSession(ctx, postgres_sqlc.CreatePublicSessionParams{
		ID:               session.ID,
		TelegramUserID:   session.TelegramUserID,
		RefreshTokenHash: session.RefreshTokenHash,
		ExpiresAt:        session.ExpiresAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("insert public session: %w", err)
	}
	return nil
}

func (s *Store) GetPublicSession(ctx context.Context, sessionID string) (domain.PublicUserSession, error) {
	row, err := s.queries.GetPublicSession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.PublicUserSession{}, fmt.Errorf("%w: public session not found", core_errors.ErrNotFound)
		}
		return domain.PublicUserSession{}, fmt.Errorf("select public session: %w", err)
	}
	return publicSessionFromRow(row), nil
}

func (s *Store) UpdatePublicSession(ctx context.Context, sessionID string, refreshTokenHash string, expiresAt time.Time) error {
	err := s.queries.UpdatePublicSession(ctx, postgres_sqlc.UpdatePublicSessionParams{
		ID:               strings.TrimSpace(sessionID),
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        expiresAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("update public session: %w", err)
	}
	return nil
}

func (s *Store) RevokePublicSession(ctx context.Context, sessionID string) error {
	err := s.queries.RevokePublicSession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return fmt.Errorf("revoke public session: %w", err)
	}
	return nil
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

func userChannelFromRow(row postgres_sqlc.TelegramUserChannel) domain.UserChannel {
	return domain.UserChannel{
		ID:             row.ID,
		TelegramUserID: row.TelegramUserID,
		ChannelID:      row.ChannelID,
		ChannelTitle:   row.ChannelTitle,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func userSettingsFromRow(row postgres_sqlc.TelegramUserSetting) domain.UserSettings {
	result := domain.UserSettings{
		TelegramUserID:              row.TelegramUserID,
		NotificationsEnabled:        row.NotificationsEnabled,
		NotificationTime:            row.NotificationTime,
		NotificationIntervalMinutes: int(row.NotificationIntervalMinutes),
		Timezone:                    row.Timezone,
		CreatedAt:                   row.CreatedAt,
		UpdatedAt:                   row.UpdatedAt,
	}
	if row.GoogleEmail.Valid {
		result.GoogleEmail = row.GoogleEmail.String
	}
	if row.GoogleRefreshToken.Valid {
		result.GoogleRefreshToken = row.GoogleRefreshToken.String
	}
	if row.GoogleConnectedAt.Valid {
		result.GoogleConnectedAt = &row.GoogleConnectedAt.Time
	}
	if row.LastNotificationSentAt.Valid {
		result.LastNotificationSentAt = &row.LastNotificationSentAt.Time
	}
	return result
}

func notificationUserFromRow(row postgres_sqlc.ListNotificationUsersRow) domain.UserSettings {
	result := domain.UserSettings{
		TelegramUserID:              row.TelegramUserID,
		NotificationsEnabled:        row.NotificationsEnabled,
		NotificationTime:            row.NotificationTime,
		NotificationIntervalMinutes: int(row.NotificationIntervalMinutes),
		Timezone:                    row.Timezone,
		CreatedAt:                   row.CreatedAt,
		UpdatedAt:                   row.UpdatedAt,
	}
	if row.LastNotificationSentAt.Valid {
		result.LastNotificationSentAt = &row.LastNotificationSentAt.Time
	}
	return result
}

func campaignFromRow(row postgres_sqlc.AdCampaign) domain.Campaign {
	result := domain.Campaign{
		ID:             row.ID,
		TelegramUserID: row.TelegramUserID,
		ChannelID:      row.ChannelID,
		ChannelTitle:   row.ChannelTitle,
		Keyword:        row.Keyword,
		StartDate:      row.StartDate,
		Timezone:       row.Timezone,
		Columns:        parseCampaignColumns(row.Columns),
		Status:         row.Status,
		ExportJWT:      row.ExportJwt,
		LastTotalViews: row.LastTotalViews,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.TargetViews.Valid {
		result.TargetViews = &row.TargetViews.Int64
	}
	if row.ClosedAt.Valid {
		result.ClosedAt = &row.ClosedAt.Time
	}
	if row.CloseReason.Valid {
		result.CloseReason = row.CloseReason.String
	}
	if row.LastSnapshotAt.Valid {
		result.LastSnapshotAt = &row.LastSnapshotAt.Time
	}
	if row.LastDailyGrowth.Valid {
		result.LastDailyGrowth = &row.LastDailyGrowth.Int64
	}
	if row.EstimatedCloseDate.Valid {
		result.EstimatedCloseDate = &row.EstimatedCloseDate.Time
	}
	if row.SpreadsheetID.Valid {
		result.SpreadsheetID = row.SpreadsheetID.String
	}
	if row.SpreadsheetUrl.Valid {
		result.SpreadsheetURL = row.SpreadsheetUrl.String
	}
	return result
}

func snapshotFromRow(row postgres_sqlc.AdCampaignSnapshot) domain.CampaignSnapshot {
	result := domain.CampaignSnapshot{
		ID:         row.ID,
		CampaignID: row.CampaignID,
		SnapshotAt: row.SnapshotAt,
		TotalViews: row.TotalViews,
		CreatedAt:  row.CreatedAt,
	}
	if row.RemainingViews.Valid {
		result.RemainingViews = &row.RemainingViews.Int64
	}
	if row.DailyGrowth.Valid {
		result.DailyGrowth = &row.DailyGrowth.Int64
	}
	if row.EstimatedCloseDate.Valid {
		result.EstimatedCloseDate = &row.EstimatedCloseDate.Time
	}
	return result
}

func inputSessionFromRow(row postgres_sqlc.TelegramInputSession) domain.CampaignInputSession {
	result := domain.CampaignInputSession{
		TelegramUserID: row.TelegramUserID,
		Flow:           row.Flow,
		Step:           row.Step,
		Payload:        string(row.Payload),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
	if row.ExpiresAt.Valid {
		result.ExpiresAt = &row.ExpiresAt.Time
	}
	return result
}

func publicSessionFromRow(row postgres_sqlc.PublicUserSession) domain.PublicUserSession {
	result := domain.PublicUserSession{
		ID:               row.ID,
		TelegramUserID:   row.TelegramUserID,
		RefreshTokenHash: row.RefreshTokenHash,
		ExpiresAt:        row.ExpiresAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
	if row.RevokedAt.Valid {
		result.RevokedAt = &row.RevokedAt.Time
	}
	return result
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: value.UTC(), Valid: true}
}

func nullableDate(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullableString(value string) sql.NullString {
	value = strings.TrimSpace(value)
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
