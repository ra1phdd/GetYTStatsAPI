package campaign_service

import (
	"context"
	"errors"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	stats_repository "getytstatsapi/internal/entities/stats/repository"
	stats_service "getytstatsapi/internal/entities/stats/service"
)

type statsRepoStub struct {
	calls  int
	videos []domain.StatsVideo
}

func (s *statsRepoStub) GetVideos(context.Context, domain.StatsQuery) ([]domain.StatsVideo, error) {
	s.calls++
	return append([]domain.StatsVideo(nil), s.videos...), nil
}

type campaignStoreStub struct {
	settings          []domain.UserSettings
	campaigns         []domain.Campaign
	latestSnapshot    domain.CampaignSnapshot
	hasLatestSnapshot bool
	markedUserID      int64
	savedSnapshots    int
	getCampaign       domain.Campaign
	userChannel       domain.UserChannel
	createdCampaign   domain.Campaign
	createCalls       int
}

func (s *campaignStoreStub) AddUserChannel(context.Context, int64, string, string) (domain.UserChannel, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) GetUserChannel(context.Context, int64, string) (domain.UserChannel, error) {
	if s.userChannel.ChannelID == "" {
		return domain.UserChannel{}, core_errors.ErrNotFound
	}
	return s.userChannel, nil
}
func (s *campaignStoreStub) ListUserChannels(context.Context, int64) ([]domain.UserChannel, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) DeleteUserChannel(context.Context, int64, string) error {
	panic("unexpected call")
}
func (s *campaignStoreStub) GetUserSettings(context.Context, int64) (domain.UserSettings, error) {
	return domain.DefaultUserSettings(1), nil
}
func (s *campaignStoreStub) UpsertUserSettings(context.Context, domain.UserSettings) (domain.UserSettings, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) SaveGoogleLink(_ context.Context, userID int64, email string, refreshToken string, connectedAt time.Time) (domain.UserSettings, error) {
	settings := domain.DefaultUserSettings(userID)
	settings.GoogleEmail = email
	settings.GoogleRefreshToken = refreshToken
	settings.GoogleConnectedAt = &connectedAt
	return settings, nil
}
func (s *campaignStoreStub) MarkNotificationSent(_ context.Context, userID int64, _ time.Time) error {
	s.markedUserID = userID
	return nil
}
func (s *campaignStoreStub) ListNotificationUsers(context.Context) ([]domain.UserSettings, error) {
	return append([]domain.UserSettings(nil), s.settings...), nil
}
func (s *campaignStoreStub) CreateCampaign(context.Context, domain.Campaign) (domain.Campaign, error) {
	s.createCalls++
	if s.createdCampaign.ID != 0 {
		return s.createdCampaign, nil
	}
	panic("unexpected call")
}
func (s *campaignStoreStub) GetCampaignByID(context.Context, int64) (domain.Campaign, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) GetCampaignByIDForUser(context.Context, int64, int64) (domain.Campaign, error) {
	return s.getCampaign, nil
}
func (s *campaignStoreStub) GetCampaignByExportJWT(context.Context, string) (domain.Campaign, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) ListUserCampaigns(context.Context, int64) ([]domain.Campaign, error) {
	return append([]domain.Campaign(nil), s.campaigns...), nil
}
func (s *campaignStoreStub) SaveCampaignSnapshot(_ context.Context, campaign domain.Campaign, snapshot domain.CampaignSnapshot) (domain.CampaignSnapshot, error) {
	s.savedSnapshots++
	snapshot.ID = int64(s.savedSnapshots)
	return snapshot, nil
}
func (s *campaignStoreStub) CloseCampaign(context.Context, int64, time.Time, string) error {
	return nil
}
func (s *campaignStoreStub) UpdateCampaignSpreadsheet(context.Context, int64, string, string) error {
	return nil
}
func (s *campaignStoreStub) GetLatestSnapshot(context.Context, int64) (domain.CampaignSnapshot, error) {
	if !s.hasLatestSnapshot {
		return domain.CampaignSnapshot{}, core_errors.ErrNotFound
	}
	return s.latestSnapshot, nil
}
func (s *campaignStoreStub) GetLatestSnapshotWithVideos(context.Context, int64) (domain.CampaignSnapshot, error) {
	return domain.CampaignSnapshot{}, errors.New("not implemented")
}
func (s *campaignStoreStub) GetPreviousSnapshot(context.Context, int64, time.Time) (domain.CampaignSnapshot, error) {
	return domain.CampaignSnapshot{}, core_errors.ErrNotFound
}
func (s *campaignStoreStub) GetInputSession(context.Context, int64) (domain.CampaignInputSession, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) UpsertInputSession(context.Context, domain.CampaignInputSession) (domain.CampaignInputSession, error) {
	panic("unexpected call")
}
func (s *campaignStoreStub) DeleteInputSession(context.Context, int64) error {
	panic("unexpected call")
}

type notifierStub struct{ digests int }

func (n *notifierStub) SendDigest(context.Context, domain.NotificationDigest) error {
	n.digests++
	return nil
}
func (n *notifierStub) SendAutoClosed(context.Context, domain.Campaign) error { return nil }

func TestProcessDueNotificationsReusesTodaySnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 12, 5, 0, 0, time.UTC)
	store := &campaignStoreStub{
		settings:          []domain.UserSettings{{TelegramUserID: 1, NotificationsEnabled: true, NotificationTime: "12:00", Timezone: "UTC"}},
		campaigns:         []domain.Campaign{{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -1), Timezone: "UTC", Status: domain.CampaignStatusActive}},
		hasLatestSnapshot: true,
		latestSnapshot:    domain.CampaignSnapshot{ID: 7, CampaignID: 10, SnapshotAt: now.Add(-2 * time.Hour), TotalViews: 123},
	}
	statsRepo := &statsRepoStub{}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }
	notifier := &notifierStub{}

	if err := service.ProcessDueNotifications(context.Background(), notifier); err != nil {
		t.Fatalf("ProcessDueNotifications() error = %v", err)
	}
	if statsRepo.calls != 0 {
		t.Fatalf("stats repo calls = %d, want 0", statsRepo.calls)
	}
	if notifier.digests != 1 {
		t.Fatalf("notifier digests = %d, want 1", notifier.digests)
	}
	if store.markedUserID != 1 {
		t.Fatalf("marked user id = %d, want 1", store.markedUserID)
	}
}

func TestRefreshCampaignForceFetchesEvenWithTodaySnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	store := &campaignStoreStub{
		getCampaign:       domain.Campaign{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -1), Timezone: "UTC", Status: domain.CampaignStatusActive},
		hasLatestSnapshot: true,
		latestSnapshot:    domain.CampaignSnapshot{ID: 7, CampaignID: 10, SnapshotAt: now.Add(-time.Hour), TotalViews: 123},
	}
	statsRepo := &statsRepoStub{videos: []domain.StatsVideo{{VideoID: "v1", Views: 50}}}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	_, _, _, err := service.RefreshCampaign(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("RefreshCampaign() error = %v", err)
	}
	if statsRepo.calls != 1 {
		t.Fatalf("stats repo calls = %d, want 1", statsRepo.calls)
	}
	if store.savedSnapshots != 1 {
		t.Fatalf("saved snapshots = %d, want 1", store.savedSnapshots)
	}
}

func TestCreateCampaignRejectsFutureDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	store := &campaignStoreStub{
		userChannel: domain.UserChannel{TelegramUserID: 1, ChannelID: "ch", ChannelTitle: "Channel"},
	}
	statsRepo := &statsRepoStub{}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	_, err := service.CreateCampaign(context.Background(), CreateCampaignRequest{
		TelegramUserID: 1,
		ChannelID:      "ch",
		Keyword:        "kw",
		StartDate:      now.AddDate(0, 0, 1),
	})
	if !errors.Is(err, core_errors.ErrInvalidArgument) {
		t.Fatalf("CreateCampaign() error = %v, want invalid argument", err)
	}
	if store.createCalls != 0 {
		t.Fatalf("create calls = %d, want 0", store.createCalls)
	}
}

func TestRefreshCampaignRequiresOneHourCooldown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 30, 0, 0, time.UTC)
	store := &campaignStoreStub{
		getCampaign: domain.Campaign{
			ID:             10,
			TelegramUserID: 1,
			ChannelID:      "ch",
			Keyword:        "kw",
			StartDate:      now.AddDate(0, 0, -1),
			Timezone:       "UTC",
			Status:         domain.CampaignStatusActive,
			LastSnapshotAt: ptrTime(now.Add(-30 * time.Minute)),
		},
	}
	statsRepo := &statsRepoStub{videos: []domain.StatsVideo{{VideoID: "v1", Views: 50}}}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	_, _, _, err := service.RefreshCampaign(context.Background(), 1, 10)
	if !errors.Is(err, core_errors.ErrConflict) {
		t.Fatalf("RefreshCampaign() error = %v, want conflict", err)
	}
	if statsRepo.calls != 0 {
		t.Fatalf("stats repo calls = %d, want 0", statsRepo.calls)
	}
}

func TestBuildChannelVerificationCode(t *testing.T) {
	t.Parallel()

	left := buildChannelVerificationCode(123, "UCabc")
	right := buildChannelVerificationCode(123, "UCabc")
	other := buildChannelVerificationCode(124, "UCabc")

	if left == "" {
		t.Fatal("buildChannelVerificationCode() = empty string")
	}
	if left != right {
		t.Fatalf("same inputs produced different codes: %q != %q", left, right)
	}
	if left == other {
		t.Fatalf("different inputs produced same code: %q", left)
	}
}

func TestBuildGoogleSheetFormula(t *testing.T) {
	t.Parallel()

	formula := buildGoogleSheetFormula("https://example.com/export.csv")
	if formula != `=IMPORTDATA("https://example.com/export.csv")` {
		t.Fatalf("buildGoogleSheetFormula() = %q", formula)
	}
}

func TestBuildCampaignSpreadsheetTitle(t *testing.T) {
	t.Parallel()

	target := int64(750000)
	title := buildCampaignSpreadsheetTitle(domain.Campaign{
		ChannelTitle: "Narezkistats Channel",
		StartDate:    time.Date(2026, time.June, 12, 0, 0, 0, 0, time.UTC),
		TargetViews:  &target,
	})
	if title != "РЕКЛАМА 750К NAREZKISTATS CHANNEL ИЮНЬ" {
		t.Fatalf("buildCampaignSpreadsheetTitle() = %q", title)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

var _ stats_repository.Repository = (*statsRepoStub)(nil)
