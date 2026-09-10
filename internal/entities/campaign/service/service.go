package campaign_service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	"getytstatsapi/internal/core/security/jwtutil"
	campaign_google "getytstatsapi/internal/entities/campaign/repository/google"
	youtube_repository "getytstatsapi/internal/entities/stats/repository/youtube"
	stats_service "getytstatsapi/internal/entities/stats/service"
)

const forcedRefreshCooldown = time.Hour

type Store interface {
	AddUserChannel(context.Context, int64, string, string) (domain.UserChannel, error)
	GetUserChannel(context.Context, int64, string) (domain.UserChannel, error)
	ListUserChannels(context.Context, int64) ([]domain.UserChannel, error)
	DeleteUserChannel(context.Context, int64, string) error
	GetUserSettings(context.Context, int64) (domain.UserSettings, error)
	UpsertUserSettings(context.Context, domain.UserSettings) (domain.UserSettings, error)
	SaveGoogleLink(context.Context, int64, string, string, time.Time) (domain.UserSettings, error)
	MarkNotificationSent(context.Context, int64, time.Time) error
	ListNotificationUsers(context.Context) ([]domain.UserSettings, error)
	CreateCampaign(context.Context, domain.Campaign) (domain.Campaign, error)
	GetCampaignByID(context.Context, int64) (domain.Campaign, error)
	GetCampaignByIDForUser(context.Context, int64, int64) (domain.Campaign, error)
	GetCampaignByExportJWT(context.Context, string) (domain.Campaign, error)
	ListUserCampaigns(context.Context, int64) ([]domain.Campaign, error)
	SaveCampaignSnapshot(context.Context, domain.Campaign, domain.CampaignSnapshot) (domain.CampaignSnapshot, error)
	CloseCampaign(context.Context, int64, time.Time, string) error
	UpdateCampaignColumns(context.Context, int64, []domain.StatsColumn) error
	UpdateCampaignTarget(context.Context, int64, *int64, string, *time.Time, string, *time.Time) error
	UpdateCampaignSpreadsheet(context.Context, int64, string, string) error
	GetLatestSnapshot(context.Context, int64) (domain.CampaignSnapshot, error)
	GetLatestSnapshotWithVideos(context.Context, int64) (domain.CampaignSnapshot, error)
	GetPreviousSnapshot(context.Context, int64, time.Time) (domain.CampaignSnapshot, error)
	GetInputSession(context.Context, int64) (domain.CampaignInputSession, error)
	UpsertInputSession(context.Context, domain.CampaignInputSession) (domain.CampaignInputSession, error)
	DeleteInputSession(context.Context, int64) error
}

type Notifier interface {
	SendDigest(context.Context, domain.NotificationDigest) error
	SendAutoClosed(context.Context, domain.Campaign) error
}

type Service struct {
	store         Store
	stats         *stats_service.Service
	youtube       *youtube_repository.Repository
	google        *campaign_google.Client
	exportSecret  string
	stateSecret   string
	publicBaseURL string
	now           func() time.Time
}

type CreateCampaignRequest struct {
	TelegramUserID int64
	ChannelID      string
	Keyword        string
	StartDate      time.Time
	TargetViews    *int64
	Columns        []domain.StatsColumn
}

type CreateCampaignDraft struct {
	ChannelID   string               `json:"channel_id,omitempty"`
	TargetViews *int64               `json:"target_views,omitempty"`
	Keyword     string               `json:"keyword,omitempty"`
	StartDate   string               `json:"start_date,omitempty"`
	Columns     []domain.StatsColumn `json:"columns,omitempty"`
}

type ChannelVerification struct {
	ChannelID        string `json:"channel_id"`
	ChannelTitle     string `json:"channel_title"`
	VerificationCode string `json:"verification_code"`
}

type GoogleLinkClaims struct {
	UserID    int64 `json:"uid"`
	ExpiresAt int64 `json:"exp"`
}

type ExportClaims struct {
	UserID      int64  `json:"uid"`
	ChannelID   string `json:"chid"`
	Keyword     string `json:"kw"`
	StartDate   string `json:"sd"`
	TargetViews *int64 `json:"tv,omitempty"`
	Version     int    `json:"ver"`
}

func New(store Store, stats *stats_service.Service, youtube *youtube_repository.Repository, google *campaign_google.Client, exportSecret string, publicBaseURL string, stateSecret string) *Service {
	return &Service{
		store:         store,
		stats:         stats,
		youtube:       youtube,
		google:        google,
		exportSecret:  strings.TrimSpace(exportSecret),
		stateSecret:   strings.TrimSpace(stateSecret),
		publicBaseURL: strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
		now:           time.Now,
	}
}

func (s *Service) AddUserChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	channelID = strings.TrimSpace(channelID)
	if userID == 0 || channelID == "" {
		return domain.UserChannel{}, fmt.Errorf("%w: user_id and channel_id are required", core_errors.ErrInvalidArgument)
	}
	channel, err := s.youtube.GetChannel(ctx, channelID)
	if err != nil {
		return domain.UserChannel{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	return s.store.AddUserChannel(ctx, userID, channel.ID, channel.Title)
}

func (s *Service) PrepareUserChannelVerification(ctx context.Context, userID int64, channelRef string) (ChannelVerification, error) {
	channelRef = strings.TrimSpace(channelRef)
	if userID == 0 || channelRef == "" {
		return ChannelVerification{}, fmt.Errorf("%w: user_id and channel reference are required", core_errors.ErrInvalidArgument)
	}
	channel, err := s.youtube.GetChannel(ctx, channelRef)
	if err != nil {
		return ChannelVerification{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	return ChannelVerification{
		ChannelID:        channel.ID,
		ChannelTitle:     channel.Title,
		VerificationCode: buildChannelVerificationCode(userID, channel.ID),
	}, nil
}

func (s *Service) VerifyAndAddUserChannel(ctx context.Context, userID int64, channelID string) (domain.UserChannel, error) {
	channelID = strings.TrimSpace(channelID)
	if userID == 0 || channelID == "" {
		return domain.UserChannel{}, fmt.Errorf("%w: user_id and channel_id are required", core_errors.ErrInvalidArgument)
	}
	channel, err := s.youtube.GetChannel(ctx, channelID)
	if err != nil {
		return domain.UserChannel{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	verificationCode := buildChannelVerificationCode(userID, channel.ID)
	if !strings.Contains(channel.Description, verificationCode) {
		return domain.UserChannel{}, fmt.Errorf("%w: verification code was not found in channel description", core_errors.ErrForbidden)
	}
	return s.store.AddUserChannel(ctx, userID, channel.ID, channel.Title)
}

func (s *Service) GetGoogleLinkURL(userID int64) (string, error) {
	if userID == 0 {
		return "", fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	if s.google == nil || !s.google.Enabled() {
		return "", fmt.Errorf("%w: google integration is not configured", core_errors.ErrConflict)
	}
	if strings.TrimSpace(s.stateSecret) == "" {
		return "", fmt.Errorf("%w: google state secret is not configured", core_errors.ErrConflict)
	}
	state, err := jwtutil.SignHS256(GoogleLinkClaims{
		UserID:    userID,
		ExpiresAt: s.now().Add(15 * time.Minute).Unix(),
	}, s.stateSecret)
	if err != nil {
		return "", fmt.Errorf("sign google state: %w", err)
	}
	return s.google.AuthURL(state)
}

func (s *Service) CompleteGoogleLink(ctx context.Context, code string, state string) (domain.UserSettings, error) {
	if s.google == nil || !s.google.Enabled() {
		return domain.UserSettings{}, fmt.Errorf("%w: google integration is not configured", core_errors.ErrConflict)
	}
	if strings.TrimSpace(s.stateSecret) == "" {
		return domain.UserSettings{}, fmt.Errorf("%w: google state secret is not configured", core_errors.ErrConflict)
	}
	var claims GoogleLinkClaims
	if err := jwtutil.ParseHS256(strings.TrimSpace(state), s.stateSecret, &claims); err != nil {
		return domain.UserSettings{}, fmt.Errorf("%w: invalid google state", core_errors.ErrUnauthorized)
	}
	if claims.UserID == 0 || claims.ExpiresAt <= s.now().Unix() {
		return domain.UserSettings{}, fmt.Errorf("%w: google state expired", core_errors.ErrUnauthorized)
	}
	account, err := s.google.ExchangeCode(ctx, strings.TrimSpace(code))
	if err != nil {
		return domain.UserSettings{}, err
	}
	return s.store.SaveGoogleLink(ctx, claims.UserID, account.Email, account.RefreshToken, s.now().UTC())
}

func (s *Service) CreateCampaignSpreadsheet(ctx context.Context, userID int64, campaignID int64) (domain.Campaign, error) {
	if userID == 0 || campaignID == 0 {
		return domain.Campaign{}, fmt.Errorf("%w: user_id and campaign_id are required", core_errors.ErrInvalidArgument)
	}
	if s.google == nil || !s.google.Enabled() {
		return domain.Campaign{}, fmt.Errorf("%w: google integration is not configured", core_errors.ErrConflict)
	}
	settings, err := s.store.GetUserSettings(ctx, userID)
	if err != nil {
		return domain.Campaign{}, err
	}
	if strings.TrimSpace(settings.GoogleRefreshToken) == "" {
		return domain.Campaign{}, fmt.Errorf("%w: google account is not linked", core_errors.ErrConflict)
	}
	item, err := s.GetCampaign(ctx, userID, campaignID)
	if err != nil {
		return domain.Campaign{}, err
	}
	if strings.TrimSpace(item.SpreadsheetURL) != "" {
		return item, nil
	}
	spreadsheet, err := s.google.CreateSpreadsheet(ctx, settings.GoogleRefreshToken, buildCampaignSpreadsheetTitle(item), buildGoogleSheetFormula(item.ExportURL(s.publicBaseURL)), item.Columns)
	if err != nil {
		return domain.Campaign{}, err
	}
	if err := s.store.UpdateCampaignSpreadsheet(ctx, item.ID, spreadsheet.ID, spreadsheet.URL); err != nil {
		return domain.Campaign{}, err
	}
	item.SpreadsheetID = spreadsheet.ID
	item.SpreadsheetURL = spreadsheet.URL
	return item, nil
}

func (s *Service) ListUserChannels(ctx context.Context, userID int64) ([]domain.UserChannel, error) {
	if userID == 0 {
		return nil, fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	return s.store.ListUserChannels(ctx, userID)
}

func (s *Service) DeleteUserChannel(ctx context.Context, userID int64, channelID string) error {
	if userID == 0 || strings.TrimSpace(channelID) == "" {
		return fmt.Errorf("%w: user_id and channel_id are required", core_errors.ErrInvalidArgument)
	}
	return s.store.DeleteUserChannel(ctx, userID, channelID)
}

func (s *Service) GetUserSettings(ctx context.Context, userID int64) (domain.UserSettings, error) {
	if userID == 0 {
		return domain.UserSettings{}, fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	return s.store.GetUserSettings(ctx, userID)
}

func (s *Service) UpdateUserSettings(ctx context.Context, settings domain.UserSettings) (domain.UserSettings, error) {
	if settings.TelegramUserID == 0 {
		return domain.UserSettings{}, fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	if _, _, err := domain.ParseNotificationTime(settings.NotificationTime); err != nil {
		return domain.UserSettings{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	settings.NotificationIntervalMinutes = domain.NormalizeNotificationIntervalMinutes(settings.NotificationIntervalMinutes)
	if err := domain.ValidateNotificationIntervalMinutes(settings.NotificationIntervalMinutes); err != nil {
		return domain.UserSettings{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}
	if strings.TrimSpace(settings.Timezone) == "" {
		settings.Timezone = domain.DefaultUserTimezone
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return domain.UserSettings{}, fmt.Errorf("%w: invalid timezone", core_errors.ErrInvalidArgument)
	}
	return s.store.UpsertUserSettings(ctx, settings)
}

func (s *Service) CreateCampaign(ctx context.Context, request CreateCampaignRequest) (domain.Campaign, error) {
	if request.TelegramUserID == 0 || strings.TrimSpace(request.ChannelID) == "" || request.Keyword == "" {
		return domain.Campaign{}, fmt.Errorf("%w: user_id, channel_id and keyword are required", core_errors.ErrInvalidArgument)
	}
	if request.TargetViews != nil && *request.TargetViews <= 0 {
		return domain.Campaign{}, fmt.Errorf("%w: target_views must be positive", core_errors.ErrInvalidArgument)
	}

	settings, err := s.store.GetUserSettings(ctx, request.TelegramUserID)
	if err != nil {
		return domain.Campaign{}, err
	}
	if strings.TrimSpace(settings.Timezone) == "" {
		settings.Timezone = domain.DefaultUserTimezone
	}
	if _, err := time.LoadLocation(settings.Timezone); err != nil {
		return domain.Campaign{}, fmt.Errorf("%w: invalid user timezone", core_errors.ErrInvalidArgument)
	}

	channel, err := s.store.GetUserChannel(ctx, request.TelegramUserID, request.ChannelID)
	if err != nil {
		return domain.Campaign{}, err
	}

	request.StartDate = normalizeDate(request.StartDate)
	loc := mustLoadLocation(settings.Timezone)
	today := time.Date(s.now().In(loc).Year(), s.now().In(loc).Month(), s.now().In(loc).Day(), 0, 0, 0, 0, loc)
	startDay := time.Date(request.StartDate.Year(), request.StartDate.Month(), request.StartDate.Day(), 0, 0, 0, 0, loc)
	if startDay.After(today) {
		return domain.Campaign{}, fmt.Errorf("%w: start_date cannot be in the future", core_errors.ErrInvalidArgument)
	}
	status := domain.CampaignStatusFor(s.now().In(mustLoadLocation(settings.Timezone)), request.StartDate.In(mustLoadLocation(settings.Timezone)), nil)
	exportToken, err := jwtutil.SignHS256(ExportClaims{
		UserID:      request.TelegramUserID,
		ChannelID:   channel.ChannelID,
		Keyword:     request.Keyword,
		StartDate:   request.StartDate.Format("2006-01-02"),
		TargetViews: request.TargetViews,
		Version:     1,
	}, s.exportSecret)
	if err != nil {
		return domain.Campaign{}, fmt.Errorf("generate export token: %w", err)
	}

	created, err := s.store.CreateCampaign(ctx, domain.Campaign{
		TelegramUserID: request.TelegramUserID,
		ChannelID:      channel.ChannelID,
		ChannelTitle:   channel.ChannelTitle,
		Keyword:        request.Keyword,
		StartDate:      request.StartDate,
		Timezone:       settings.Timezone,
		TargetViews:    request.TargetViews,
		Columns:        domain.NormalizeStatsColumns(request.Columns),
		Status:         status,
		ExportJWT:      exportToken,
	})
	if err != nil {
		return domain.Campaign{}, err
	}

	if refreshed, _, _, refreshErr := s.refreshCampaign(ctx, created, false); refreshErr == nil {
		return refreshed, nil
	}

	return created, nil
}

func (s *Service) ListCampaigns(ctx context.Context, userID int64, status string, page int, pageSize int) (domain.CampaignList, error) {
	if userID == 0 {
		return domain.CampaignList{}, fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = domain.DefaultCampaignsPageSize
	}

	items, err := s.store.ListUserCampaigns(ctx, userID)
	if err != nil {
		return domain.CampaignList{}, err
	}

	filtered := make([]domain.Campaign, 0, len(items))
	now := s.now()
	status = strings.TrimSpace(strings.ToLower(status))
	for _, item := range items {
		item.Status = s.resolveStatus(now, item)
		if status != "" && status != "all" && item.Status != status {
			continue
		}
		filtered = append(filtered, item)
	}

	start := (page - 1) * pageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return domain.CampaignList{
		Items:    filtered[start:end],
		Page:     page,
		PageSize: pageSize,
		HasPrev:  page > 1 && len(filtered) > 0,
		HasNext:  end < len(filtered),
		Total:    len(filtered),
	}, nil
}

func (s *Service) GetCampaign(ctx context.Context, userID int64, campaignID int64) (domain.Campaign, error) {
	if userID == 0 || campaignID == 0 {
		return domain.Campaign{}, fmt.Errorf("%w: user_id and campaign_id are required", core_errors.ErrInvalidArgument)
	}
	item, err := s.store.GetCampaignByIDForUser(ctx, campaignID, userID)
	if err != nil {
		return domain.Campaign{}, err
	}
	item.Status = s.resolveStatus(s.now(), item)
	return item, nil
}

func (s *Service) CloseCampaign(ctx context.Context, userID int64, campaignID int64) (domain.Campaign, error) {
	item, err := s.GetCampaign(ctx, userID, campaignID)
	if err != nil {
		return domain.Campaign{}, err
	}
	if item.ClosedAt != nil {
		return domain.Campaign{}, fmt.Errorf("%w: campaign is already closed", core_errors.ErrConflict)
	}
	closedAt := s.now().UTC()
	if err := s.store.CloseCampaign(ctx, item.ID, closedAt, domain.CampaignCloseReasonManual); err != nil {
		return domain.Campaign{}, err
	}
	item.ClosedAt = &closedAt
	item.CloseReason = domain.CampaignCloseReasonManual
	item.Status = domain.CampaignStatusClosed
	return item, nil
}

func (s *Service) UpdateCampaignColumns(ctx context.Context, userID int64, campaignID int64, columns []domain.StatsColumn) (domain.Campaign, error) {
	item, err := s.GetCampaign(ctx, userID, campaignID)
	if err != nil {
		return domain.Campaign{}, err
	}
	columns = domain.NormalizeStatsColumns(columns)
	if err := s.store.UpdateCampaignColumns(ctx, item.ID, columns); err != nil {
		return domain.Campaign{}, err
	}
	item.Columns = columns
	return item, nil
}

func (s *Service) UpdateCampaignTarget(ctx context.Context, userID int64, campaignID int64, targetViews *int64) (domain.Campaign, error) {
	item, err := s.GetCampaign(ctx, userID, campaignID)
	if err != nil {
		return domain.Campaign{}, err
	}
	if targetViews != nil && *targetViews <= 0 {
		return domain.Campaign{}, fmt.Errorf("%w: target_views must be positive", core_errors.ErrInvalidArgument)
	}
	item.TargetViews = targetViews
	item = s.recalculateTargetState(item)
	if err := s.store.UpdateCampaignTarget(ctx, item.ID, item.TargetViews, item.Status, item.ClosedAt, item.CloseReason, item.EstimatedCloseDate); err != nil {
		return domain.Campaign{}, err
	}
	return item, nil
}

func (s *Service) RefreshCampaign(ctx context.Context, userID int64, campaignID int64) (domain.Campaign, domain.CampaignSnapshot, bool, error) {
	item, err := s.GetCampaign(ctx, userID, campaignID)
	if err != nil {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
	}
	if wait := refreshCooldownRemaining(item, s.now()); wait > 0 {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, fmt.Errorf("%w: campaign can be refreshed again in %s", core_errors.ErrConflict, humanizeDuration(wait))
	}
	return s.refreshCampaign(ctx, item, true)
}

func (s *Service) ExportCampaignCSV(ctx context.Context, token string) ([]byte, string, error) {
	var claims ExportClaims
	if err := jwtutil.ParseHS256(token, s.exportSecret, &claims); err != nil {
		return nil, "", fmt.Errorf("%w: invalid export token", core_errors.ErrUnauthorized)
	}

	item, err := s.store.GetCampaignByExportJWT(ctx, strings.TrimSpace(token))
	if err != nil {
		return nil, "", err
	}
	item.Status = s.resolveStatus(s.now(), item)

	if !s.started(item, s.now()) {
		data, err := buildStatusCSV("РК еще не началась")
		return data, item.Title(), err
	}

	snapshot, err := s.store.GetLatestSnapshotWithVideos(ctx, item.ID)
	if err != nil {
		if item.Status != domain.CampaignStatusClosed {
			item, snapshot, _, err = s.refreshCampaign(ctx, item, false)
			if err != nil {
				return nil, "", err
			}
		} else {
			return nil, "", err
		}
	}

	columns := domain.NormalizeStatsColumns(item.Columns)
	snapshot.Videos, err = s.stats.PopulateSponsorSegments(ctx, snapshot.Videos, columns)
	if err != nil {
		return nil, "", err
	}
	data, err := s.stats.BuildCSV(snapshot.Videos, columns)
	if err != nil {
		return nil, "", err
	}
	return data, item.Title(), nil
}

func (s *Service) GetInputSession(ctx context.Context, userID int64) (domain.CampaignInputSession, error) {
	if userID == 0 {
		return domain.CampaignInputSession{}, fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	return s.store.GetInputSession(ctx, userID)
}

func (s *Service) UpsertInputSession(ctx context.Context, session domain.CampaignInputSession) (domain.CampaignInputSession, error) {
	if session.TelegramUserID == 0 || strings.TrimSpace(session.Flow) == "" || strings.TrimSpace(session.Step) == "" {
		return domain.CampaignInputSession{}, fmt.Errorf("%w: user_id, flow and step are required", core_errors.ErrInvalidArgument)
	}
	return s.store.UpsertInputSession(ctx, session)
}

func (s *Service) DeleteInputSession(ctx context.Context, userID int64) error {
	if userID == 0 {
		return fmt.Errorf("%w: user_id is required", core_errors.ErrInvalidArgument)
	}
	return s.store.DeleteInputSession(ctx, userID)
}

func (s *Service) ParseDraftPayload(payload string) (CreateCampaignDraft, error) {
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return CreateCampaignDraft{}, nil
	}
	var draft CreateCampaignDraft
	if err := json.Unmarshal([]byte(payload), &draft); err != nil {
		return CreateCampaignDraft{}, fmt.Errorf("parse draft payload: %w", err)
	}
	return draft, nil
}

func (s *Service) BuildDraftPayload(draft CreateCampaignDraft) (string, error) {
	data, err := json.Marshal(draft)
	if err != nil {
		return "", fmt.Errorf("build draft payload: %w", err)
	}
	return string(data), nil
}

func (s *Service) recalculateTargetState(item domain.Campaign) domain.Campaign {
	if item.CloseReason == domain.CampaignCloseReasonManual && item.ClosedAt != nil {
		item.Status = domain.CampaignStatusClosed
		return item
	}

	if item.TargetViews == nil {
		item.ClosedAt = nil
		item.CloseReason = ""
		item.EstimatedCloseDate = nil
		item.Status = s.resolveStatus(s.now(), item)
		return item
	}

	remaining := *item.TargetViews - item.LastTotalViews
	if remaining <= 0 {
		closedAt := s.now().UTC()
		if item.LastSnapshotAt != nil {
			closedAt = item.LastSnapshotAt.UTC()
		}
		item.ClosedAt = &closedAt
		item.CloseReason = domain.CampaignCloseReasonTargetReached
		item.EstimatedCloseDate = nil
		item.Status = domain.CampaignStatusClosed
		return item
	}

	item.ClosedAt = nil
	item.CloseReason = ""
	item.EstimatedCloseDate = nil
	if item.LastDailyGrowth != nil && *item.LastDailyGrowth > 0 && item.LastSnapshotAt != nil {
		if days := domain.EstimateCloseInDays(remaining, *item.LastDailyGrowth); days != nil {
			date := item.LastSnapshotAt.AddDate(0, 0, int(*days))
			item.EstimatedCloseDate = &date
		}
	}
	item.Status = s.resolveStatus(s.now(), item)
	return item
}

func (s *Service) ProcessDueNotifications(ctx context.Context, notifier Notifier) error {
	if notifier == nil {
		return nil
	}
	settingsList, err := s.store.ListNotificationUsers(ctx)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	for _, settings := range settingsList {
		due, err := s.isNotificationDue(now, settings)
		if err != nil || !due {
			continue
		}

		campaigns, err := s.store.ListUserCampaigns(ctx, settings.TelegramUserID)
		if err != nil {
			return err
		}

		digest := domain.NotificationDigest{
			UserID:           settings.TelegramUserID,
			NotificationDate: now,
			Campaigns:        make([]domain.NotificationDigestCampaign, 0),
		}

		for _, campaign := range campaigns {
			campaign.Status = s.resolveStatus(now, campaign)
			if campaign.Status != domain.CampaignStatusActive {
				continue
			}

			updatedCampaign, snapshot, autoClosed, err := s.refreshCampaign(ctx, campaign, false)
			if err != nil {
				return err
			}
			if autoClosed {
				if err := notifier.SendAutoClosed(ctx, updatedCampaign); err != nil {
					return err
				}
			}

			item := domain.NotificationDigestCampaign{
				CampaignID:         updatedCampaign.ID,
				Title:              updatedCampaign.Title(),
				Status:             updatedCampaign.Status,
				TotalViews:         snapshot.TotalViews,
				RemainingViews:     snapshot.RemainingViews,
				DailyGrowth:        snapshot.DailyGrowth,
				EstimatedCloseDate: snapshot.EstimatedCloseDate,
				ExportURL:          updatedCampaign.ExportURL(s.publicBaseURL),
			}
			if snapshot.RemainingViews != nil && snapshot.DailyGrowth != nil {
				item.EstimatedCloseInDays = domain.EstimateCloseInDays(*snapshot.RemainingViews, *snapshot.DailyGrowth)
			}
			digest.Campaigns = append(digest.Campaigns, item)
		}

		if len(digest.Campaigns) > 0 {
			if err := notifier.SendDigest(ctx, digest); err != nil {
				return err
			}
		}
		if err := s.store.MarkNotificationSent(ctx, settings.TelegramUserID, now); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) refreshCampaign(ctx context.Context, campaign domain.Campaign, force bool) (domain.Campaign, domain.CampaignSnapshot, bool, error) {
	campaign.Status = s.resolveStatus(s.now(), campaign)
	if campaign.Status == domain.CampaignStatusClosed {
		snapshot, err := s.store.GetLatestSnapshot(ctx, campaign.ID)
		if err != nil {
			return campaign, domain.CampaignSnapshot{}, false, err
		}
		return campaign, snapshot, false, nil
	}
	if !s.started(campaign, s.now()) {
		return campaign, domain.CampaignSnapshot{}, false, nil
	}
	if !force {
		snapshot, ok, err := s.snapshotForToday(ctx, campaign)
		if err != nil {
			return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
		}
		if ok && canReuseSnapshot(snapshot, s.now()) {
			campaign.LastSnapshotAt = &snapshot.SnapshotAt
			campaign.LastTotalViews = snapshot.TotalViews
			campaign.LastDailyGrowth = snapshot.DailyGrowth
			campaign.EstimatedCloseDate = snapshot.EstimatedCloseDate
			return campaign, snapshot, false, nil
		}
	}

	startAt, err := campaignStartTime(campaign)
	if err != nil {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
	}

	videos, err := s.stats.GetVideos(ctx, domain.NewStatsQuery(
		campaign.ChannelID,
		campaign.Keyword,
		startAt,
		s.now().UTC(),
		nil,
		domain.DefaultStatsColumns(),
	))
	if err != nil {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
	}

	totalViews := int64(0)
	for _, video := range videos {
		totalViews += int64(video.Views)
	}

	snapshotTime := s.now().UTC()
	var dailyGrowth *int64
	if baseline, normalize, ok, err := s.dailyGrowthBaselineSnapshot(ctx, campaign, snapshotTime); err != nil {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
	} else if ok {
		dailyGrowth = calculateDailyGrowth(totalViews, snapshotTime, baseline, normalize)
	}

	var remaining *int64
	var estimatedCloseDate *time.Time
	if campaign.TargetViews != nil {
		value := *campaign.TargetViews - totalViews
		if value < 0 {
			value = 0
		}
		remaining = &value
		if dailyGrowth != nil {
			if days := domain.EstimateCloseInDays(value, *dailyGrowth); days != nil {
				date := snapshotTime.AddDate(0, 0, int(*days))
				estimatedCloseDate = &date
			}
		}
	}

	autoClosed := false
	if campaign.TargetViews != nil && totalViews >= *campaign.TargetViews {
		closedAt := snapshotTime
		campaign.ClosedAt = &closedAt
		campaign.CloseReason = domain.CampaignCloseReasonTargetReached
		campaign.Status = domain.CampaignStatusClosed
		autoClosed = true
	} else {
		campaign.Status = s.resolveStatus(snapshotTime, campaign)
	}

	snapshot, err := s.store.SaveCampaignSnapshot(ctx, campaign, domain.CampaignSnapshot{
		CampaignID:         campaign.ID,
		SnapshotAt:         snapshotTime,
		TotalViews:         totalViews,
		RemainingViews:     remaining,
		DailyGrowth:        dailyGrowth,
		EstimatedCloseDate: estimatedCloseDate,
		Videos:             videos,
	})
	if err != nil {
		return domain.Campaign{}, domain.CampaignSnapshot{}, false, err
	}
	campaign.LastSnapshotAt = &snapshot.SnapshotAt
	campaign.LastTotalViews = snapshot.TotalViews
	campaign.LastDailyGrowth = snapshot.DailyGrowth
	campaign.EstimatedCloseDate = snapshot.EstimatedCloseDate
	return campaign, snapshot, autoClosed, nil
}

func (s *Service) snapshotForToday(ctx context.Context, campaign domain.Campaign) (domain.CampaignSnapshot, bool, error) {
	snapshot, err := s.store.GetLatestSnapshot(ctx, campaign.ID)
	if err != nil {
		if errors.Is(err, core_errors.ErrNotFound) {
			return domain.CampaignSnapshot{}, false, nil
		}
		return domain.CampaignSnapshot{}, false, err
	}
	loc := mustLoadLocation(campaign.Timezone)
	if sameDay(snapshot.SnapshotAt.In(loc), s.now().In(loc)) {
		return snapshot, true, nil
	}
	return domain.CampaignSnapshot{}, false, nil
}

func sameDay(left time.Time, right time.Time) bool {
	return left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()
}

func (s *Service) resolveStatus(now time.Time, campaign domain.Campaign) string {
	loc := mustLoadLocation(campaign.Timezone)
	start := time.Date(campaign.StartDate.Year(), campaign.StartDate.Month(), campaign.StartDate.Day(), 0, 0, 0, 0, loc)
	return domain.CampaignStatusFor(now.In(loc), start, campaign.ClosedAt)
}

func (s *Service) started(campaign domain.Campaign, now time.Time) bool {
	loc := mustLoadLocation(campaign.Timezone)
	start := time.Date(campaign.StartDate.Year(), campaign.StartDate.Month(), campaign.StartDate.Day(), 0, 0, 0, 0, loc)
	nowLocal := now.In(loc)
	today := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)
	return !start.After(today)
}

func (s *Service) isNotificationDue(now time.Time, settings domain.UserSettings) (bool, error) {
	intervalMinutes := domain.NormalizeNotificationIntervalMinutes(settings.NotificationIntervalMinutes)
	if err := domain.ValidateNotificationIntervalMinutes(intervalMinutes); err != nil {
		return false, err
	}
	interval := time.Duration(intervalMinutes) * time.Minute
	loc := mustLoadLocation(settings.Timezone)
	hour, minute, err := domain.ParseNotificationTime(settings.NotificationTime)
	if err != nil {
		return false, err
	}
	nowLocal := now.In(loc)
	currentAnchor := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), hour, minute, 0, 0, loc)
	if interval%(24*time.Hour) != 0 {
		if settings.LastNotificationSentAt == nil {
			return true, nil
		}
		return settings.LastNotificationSentAt.UTC().Add(interval).Before(now) || settings.LastNotificationSentAt.UTC().Add(interval).Equal(now), nil
	}
	if settings.LastNotificationSentAt == nil {
		return !nowLocal.Before(currentAnchor), nil
	}
	lastLocal := settings.LastNotificationSentAt.In(loc)
	lastAnchor := time.Date(lastLocal.Year(), lastLocal.Month(), lastLocal.Day(), hour, minute, 0, 0, loc)
	nextAnchor := lastAnchor
	if !lastLocal.Before(lastAnchor) {
		nextAnchor = nextAnchor.Add(interval)
	}
	return !nowLocal.Before(nextAnchor), nil
}

func campaignStartTime(campaign domain.Campaign) (time.Time, error) {
	loc, err := time.LoadLocation(campaign.Timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid campaign timezone", core_errors.ErrInvalidArgument)
	}
	local := time.Date(campaign.StartDate.Year(), campaign.StartDate.Month(), campaign.StartDate.Day(), 0, 0, 0, 0, loc)
	return local.UTC(), nil
}

func normalizeDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func refreshCooldownRemaining(campaign domain.Campaign, now time.Time) time.Duration {
	if campaign.LastSnapshotAt == nil {
		return 0
	}
	remaining := campaign.LastSnapshotAt.Add(forcedRefreshCooldown).Sub(now.UTC())
	if remaining < 0 {
		return 0
	}
	return remaining
}

func canReuseSnapshot(snapshot domain.CampaignSnapshot, now time.Time) bool {
	return !snapshot.SnapshotAt.Add(forcedRefreshCooldown).Before(now.UTC())
}

func (s *Service) dailyGrowthBaselineSnapshot(ctx context.Context, campaign domain.Campaign, snapshotTime time.Time) (domain.CampaignSnapshot, bool, bool, error) {
	loc := mustLoadLocation(campaign.Timezone)
	todayStart := startOfDay(snapshotTime.In(loc))
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	if candidate, err := s.store.GetPreviousSnapshot(ctx, campaign.ID, todayStart.UTC()); err == nil {
		candidateDay := startOfDay(candidate.SnapshotAt.In(loc))
		if candidateDay.Equal(yesterdayStart) {
			return candidate, true, true, nil
		}
	} else if !errors.Is(err, core_errors.ErrNotFound) {
		return domain.CampaignSnapshot{}, false, false, err
	}

	candidate, err := s.store.GetPreviousSnapshot(ctx, campaign.ID, snapshotTime)
	if err != nil {
		if errors.Is(err, core_errors.ErrNotFound) {
			return domain.CampaignSnapshot{}, false, false, nil
		}
		return domain.CampaignSnapshot{}, false, false, err
	}
	if snapshotTime.Sub(candidate.SnapshotAt) >= 24*time.Hour {
		return domain.CampaignSnapshot{}, false, false, nil
	}
	return candidate, true, true, nil
}

func calculateDailyGrowth(totalViews int64, snapshotTime time.Time, previous domain.CampaignSnapshot, normalize bool) *int64 {
	change := totalViews - previous.TotalViews
	if !normalize {
		return &change
	}
	interval := snapshotTime.Sub(previous.SnapshotAt)
	if interval <= 0 {
		return nil
	}
	value := int64(math.Round(float64(change) * float64(24*time.Hour) / float64(interval)))
	return &value
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func humanizeDuration(value time.Duration) string {
	if value <= 0 {
		return "0m"
	}
	minutes := int(value.Round(time.Minute) / time.Minute)
	if minutes <= 0 {
		minutes = 1
	}
	hours := minutes / 60
	minutes = minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func mustLoadLocation(name string) *time.Location {
	if strings.TrimSpace(name) == "" {
		name = domain.DefaultUserTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("UTC", 0)
	}
	return loc
}

func buildStatusCSV(message string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"message"}); err != nil {
		return nil, err
	}
	if err := writer.Write([]string{message}); err != nil {
		return nil, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildChannelVerificationCode(userID int64, channelID string) string {
	payload := fmt.Sprintf("%d:%s", userID, strings.TrimSpace(channelID))
	sum := sha256.Sum256([]byte(payload))
	return "gys-verify-" + hex.EncodeToString(sum[:8])
}

func buildGoogleSheetFormula(exportURL string) string {
	exportURL = strings.TrimSpace(exportURL)
	if exportURL == "" {
		return ""
	}
	return fmt.Sprintf("=IMPORTDATA(%q)", exportURL)
}

func buildCampaignSpreadsheetTitle(item domain.Campaign) string {
	parts := []string{"РЕКЛАМА"}
	if item.TargetViews != nil {
		parts = append(parts, normalizeSpreadsheetTargetLabel(domain.FormatViewsTarget(*item.TargetViews)))
	}
	channelTitle := strings.ToUpper(strings.TrimSpace(item.ChannelTitle))
	if channelTitle != "" {
		parts = append(parts, channelTitle)
	}
	month := russianMonthName(item.StartDate.Month())
	if month != "" {
		parts = append(parts, month)
	}
	return strings.Join(parts, " ")
}

func russianMonthName(month time.Month) string {
	switch month {
	case time.January:
		return "ЯНВАРЬ"
	case time.February:
		return "ФЕВРАЛЬ"
	case time.March:
		return "МАРТ"
	case time.April:
		return "АПРЕЛЬ"
	case time.May:
		return "МАЙ"
	case time.June:
		return "ИЮНЬ"
	case time.July:
		return "ИЮЛЬ"
	case time.August:
		return "АВГУСТ"
	case time.September:
		return "СЕНТЯБРЬ"
	case time.October:
		return "ОКТЯБРЬ"
	case time.November:
		return "НОЯБРЬ"
	case time.December:
		return "ДЕКАБРЬ"
	default:
		return ""
	}
}

func normalizeSpreadsheetTargetLabel(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "K", "К")
	return value
}
