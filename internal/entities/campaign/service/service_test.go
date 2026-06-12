package campaign_service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	"getytstatsapi/internal/core/security/jwtutil"
	sponsorblock_repository "getytstatsapi/internal/entities/sponsorblock/repository"
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
	settings            []domain.UserSettings
	campaigns           []domain.Campaign
	latestSnapshot      domain.CampaignSnapshot
	latestSnapshotCSV   domain.CampaignSnapshot
	previousSnapshot    domain.CampaignSnapshot
	previousSnapshots   []domain.CampaignSnapshot
	hasLatestSnapshot   bool
	hasPreviousSnapshot bool
	markedUserID        int64
	savedSnapshots      int
	lastSavedSnapshot   domain.CampaignSnapshot
	getCampaign         domain.Campaign
	exportCampaign      domain.Campaign
	userChannel         domain.UserChannel
	createdCampaign     domain.Campaign
	createCalls         int
}

type sponsorBlockRepoStub struct {
	segments map[string][]domain.SponsorBlockSegment
	calls    int
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
	if s.exportCampaign.ID == 0 {
		panic("unexpected call")
	}
	return s.exportCampaign, nil
}
func (s *campaignStoreStub) ListUserCampaigns(context.Context, int64) ([]domain.Campaign, error) {
	return append([]domain.Campaign(nil), s.campaigns...), nil
}
func (s *campaignStoreStub) SaveCampaignSnapshot(_ context.Context, campaign domain.Campaign, snapshot domain.CampaignSnapshot) (domain.CampaignSnapshot, error) {
	s.savedSnapshots++
	s.lastSavedSnapshot = snapshot
	snapshot.ID = int64(s.savedSnapshots)
	return snapshot, nil
}
func (s *campaignStoreStub) CloseCampaign(context.Context, int64, time.Time, string) error {
	return nil
}
func (s *campaignStoreStub) UpdateCampaignColumns(context.Context, int64, []domain.StatsColumn) error {
	return nil
}
func (s *campaignStoreStub) UpdateCampaignTarget(context.Context, int64, *int64, string, *time.Time, string, *time.Time) error {
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
	if s.latestSnapshotCSV.ID == 0 {
		return domain.CampaignSnapshot{}, errors.New("not implemented")
	}
	return s.latestSnapshotCSV, nil
}
func (s *campaignStoreStub) GetPreviousSnapshot(_ context.Context, _ int64, before time.Time) (domain.CampaignSnapshot, error) {
	if len(s.previousSnapshots) > 0 {
		var selected *domain.CampaignSnapshot
		for idx := range s.previousSnapshots {
			candidate := s.previousSnapshots[idx]
			if !candidate.SnapshotAt.Before(before) {
				continue
			}
			if selected == nil || candidate.SnapshotAt.After(selected.SnapshotAt) {
				selected = &candidate
			}
		}
		if selected != nil {
			return *selected, nil
		}
	}
	if s.hasPreviousSnapshot {
		return s.previousSnapshot, nil
	}
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

func (s *sponsorBlockRepoStub) GetSkipSegments(_ context.Context, videoID string) ([]domain.SponsorBlockSegment, error) {
	s.calls++
	segments := s.segments[videoID]
	return append([]domain.SponsorBlockSegment(nil), segments...), nil
}

func TestProcessDueNotificationsReusesRecentSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 12, 5, 0, 0, time.UTC)
	store := &campaignStoreStub{
		settings:          []domain.UserSettings{{TelegramUserID: 1, NotificationsEnabled: true, NotificationTime: "12:00", NotificationIntervalMinutes: int(domain.DefaultNotificationInterval / time.Minute), Timezone: "UTC"}},
		campaigns:         []domain.Campaign{{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -1), Timezone: "UTC", Status: domain.CampaignStatusActive}},
		hasLatestSnapshot: true,
		latestSnapshot:    domain.CampaignSnapshot{ID: 7, CampaignID: 10, SnapshotAt: now.Add(-30 * time.Minute), TotalViews: 123},
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

func TestProcessDueNotificationsRefreshesStaleTodaySnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 12, 5, 0, 0, time.UTC)
	store := &campaignStoreStub{
		settings:          []domain.UserSettings{{TelegramUserID: 1, NotificationsEnabled: true, NotificationTime: "12:00", NotificationIntervalMinutes: int(domain.DefaultNotificationInterval / time.Minute), Timezone: "UTC"}},
		campaigns:         []domain.Campaign{{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -1), Timezone: "UTC", Status: domain.CampaignStatusActive}},
		hasLatestSnapshot: true,
		latestSnapshot:    domain.CampaignSnapshot{ID: 7, CampaignID: 10, SnapshotAt: now.Add(-2 * time.Hour), TotalViews: 123},
	}
	statsRepo := &statsRepoStub{videos: []domain.StatsVideo{{VideoID: "v1", Views: 50}}}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }
	notifier := &notifierStub{}

	if err := service.ProcessDueNotifications(context.Background(), notifier); err != nil {
		t.Fatalf("ProcessDueNotifications() error = %v", err)
	}
	if statsRepo.calls != 1 {
		t.Fatalf("stats repo calls = %d, want 1", statsRepo.calls)
	}
	if store.savedSnapshots != 1 {
		t.Fatalf("saved snapshots = %d, want 1", store.savedSnapshots)
	}
	if notifier.digests != 1 {
		t.Fatalf("notifier digests = %d, want 1", notifier.digests)
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

func TestRefreshCampaignNormalizesDailyGrowthToPerDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	store := &campaignStoreStub{
		getCampaign:         domain.Campaign{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -1), Timezone: "UTC", Status: domain.CampaignStatusActive},
		hasPreviousSnapshot: true,
		previousSnapshot: domain.CampaignSnapshot{
			ID:         7,
			CampaignID: 10,
			SnapshotAt: now.Add(-2 * time.Hour),
			TotalViews: 100,
		},
	}
	statsRepo := &statsRepoStub{videos: []domain.StatsVideo{{VideoID: "v1", Views: 200}}}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	updated, snapshot, _, err := service.RefreshCampaign(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("RefreshCampaign() error = %v", err)
	}
	if snapshot.DailyGrowth == nil {
		t.Fatal("snapshot.DailyGrowth = nil, want value")
	}
	if *snapshot.DailyGrowth != 1200 {
		t.Fatalf("snapshot.DailyGrowth = %d, want 1200", *snapshot.DailyGrowth)
	}
	if store.lastSavedSnapshot.DailyGrowth == nil || *store.lastSavedSnapshot.DailyGrowth != 1200 {
		t.Fatalf("saved snapshot daily growth = %v, want 1200", store.lastSavedSnapshot.DailyGrowth)
	}
	if updated.LastDailyGrowth == nil || *updated.LastDailyGrowth != 1200 {
		t.Fatalf("updated.LastDailyGrowth = %v, want 1200", updated.LastDailyGrowth)
	}
}

func TestRefreshCampaignUsesPreviousDaySnapshotForDailyGrowth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	store := &campaignStoreStub{
		getCampaign: domain.Campaign{ID: 10, TelegramUserID: 1, ChannelID: "ch", Keyword: "kw", StartDate: now.AddDate(0, 0, -2), Timezone: "UTC", Status: domain.CampaignStatusActive},
		previousSnapshots: []domain.CampaignSnapshot{
			{ID: 7, CampaignID: 10, SnapshotAt: now.Add(-2 * time.Hour), TotalViews: 190},
			{ID: 6, CampaignID: 10, SnapshotAt: now.Add(-23 * time.Hour), TotalViews: 100},
		},
	}
	statsRepo := &statsRepoStub{videos: []domain.StatsVideo{{VideoID: "v1", Views: 200}}}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	updated, snapshot, _, err := service.RefreshCampaign(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("RefreshCampaign() error = %v", err)
	}
	if snapshot.DailyGrowth == nil {
		t.Fatal("snapshot.DailyGrowth = nil, want value")
	}
	if *snapshot.DailyGrowth != 100 {
		t.Fatalf("snapshot.DailyGrowth = %d, want 100", *snapshot.DailyGrowth)
	}
	if store.lastSavedSnapshot.DailyGrowth == nil || *store.lastSavedSnapshot.DailyGrowth != 100 {
		t.Fatalf("saved snapshot daily growth = %v, want 100", store.lastSavedSnapshot.DailyGrowth)
	}
	if updated.LastDailyGrowth == nil || *updated.LastDailyGrowth != 100 {
		t.Fatalf("updated.LastDailyGrowth = %v, want 100", updated.LastDailyGrowth)
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

func TestGetCampaignAllowsAccessToAnotherManagersCampaign(t *testing.T) {
	t.Parallel()

	store := &campaignStoreStub{
		getCampaign: domain.Campaign{
			ID:             10,
			TelegramUserID: 2,
			ChannelID:      "ch",
			Keyword:        "kw",
			StartDate:      time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			Timezone:       "UTC",
			Status:         domain.CampaignStatusActive,
		},
	}
	service := New(store, stats_service.New(&statsRepoStub{}, nil), nil, nil, "secret", "http://localhost", "state-secret")

	item, err := service.GetCampaign(context.Background(), 1, 10)
	if err != nil {
		t.Fatalf("GetCampaign() error = %v", err)
	}
	if item.TelegramUserID != 2 {
		t.Fatalf("TelegramUserID = %d, want 2", item.TelegramUserID)
	}
}

func TestListCampaignsIncludesAnotherManagersChannelCampaigns(t *testing.T) {
	t.Parallel()

	store := &campaignStoreStub{campaigns: []domain.Campaign{
		{
			ID:             10,
			TelegramUserID: 2,
			ChannelID:      "ch",
			Keyword:        "kw",
			StartDate:      time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC),
			Timezone:       "UTC",
			Status:         domain.CampaignStatusActive,
		},
	}}
	service := New(store, stats_service.New(&statsRepoStub{}, nil), nil, nil, "secret", "http://localhost", "state-secret")

	list, err := service.ListCampaigns(context.Background(), 1, "", 1, 10)
	if err != nil {
		t.Fatalf("ListCampaigns() error = %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("len(ListCampaigns().Items) = %d, want 1", len(list.Items))
	}
	if list.Items[0].TelegramUserID != 2 {
		t.Fatalf("ListCampaigns().Items[0].TelegramUserID = %d, want 2", list.Items[0].TelegramUserID)
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

func TestExportCampaignCSVPopulatesSponsorSegmentsFromSnapshotVideos(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	token, err := jwtutil.SignHS256(ExportClaims{
		UserID:    1,
		ChannelID: "ch",
		Keyword:   "kw",
		StartDate: "2026-06-11",
		Version:   1,
	}, "secret")
	if err != nil {
		t.Fatalf("SignHS256() error = %v", err)
	}

	store := &campaignStoreStub{
		exportCampaign: domain.Campaign{
			ID:             10,
			TelegramUserID: 1,
			ChannelID:      "ch",
			ChannelTitle:   "Channel",
			Keyword:        "kw",
			StartDate:      now.AddDate(0, 0, -1),
			Timezone:       "UTC",
			ExportJWT:      token,
			Columns: []domain.StatsColumn{
				domain.StatsColumnID,
				domain.StatsColumnAdTimings,
			},
		},
		latestSnapshotCSV: domain.CampaignSnapshot{
			ID:         7,
			CampaignID: 10,
			Videos: []domain.StatsVideo{{
				VideoID:     "IT66BIq4Zmg",
				PublishDate: now.Add(-24 * time.Hour),
				Views:       10,
				URL:         "https://youtube.com/watch?v=IT66BIq4Zmg",
			}},
		},
	}
	sponsorRepo := &sponsorBlockRepoStub{segments: map[string][]domain.SponsorBlockSegment{
		"IT66BIq4Zmg": {
			domain.NewSponsorBlockSegment("10a88ab5", "sponsor", "skip", 137.735, 161.146, 3553.561, 0, 2, ""),
		},
	}}
	stats := stats_service.New(nil, sponsorRepo)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	data, _, err := service.ExportCampaignCSV(context.Background(), token)
	if err != nil {
		t.Fatalf("ExportCampaignCSV() error = %v", err)
	}
	if sponsorRepo.calls != 1 {
		t.Fatalf("sponsor repo calls = %d, want 1", sponsorRepo.calls)
	}
	if got := string(data); !strings.Contains(got, "02:17-02:41") {
		t.Fatalf("ExportCampaignCSV() missing sponsor timings, got %q", got)
	}
}

func TestUpdateCampaignTargetReopensTargetReachedCampaign(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	closedAt := now.Add(-2 * time.Hour)
	lastSnapshotAt := now.Add(-3 * time.Hour)
	oldTarget := int64(500000)
	newTarget := int64(750000)
	store := &campaignStoreStub{
		getCampaign: domain.Campaign{
			ID:                 10,
			TelegramUserID:     1,
			ChannelID:          "ch",
			Keyword:            "kw",
			StartDate:          now.AddDate(0, 0, -1),
			Timezone:           "UTC",
			TargetViews:        &oldTarget,
			Status:             domain.CampaignStatusClosed,
			ClosedAt:           &closedAt,
			CloseReason:        domain.CampaignCloseReasonTargetReached,
			LastSnapshotAt:     &lastSnapshotAt,
			LastTotalViews:     600000,
			LastDailyGrowth:    ptrInt64(50000),
			EstimatedCloseDate: &closedAt,
		},
	}
	statsRepo := &statsRepoStub{}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	updated, err := service.UpdateCampaignTarget(context.Background(), 1, 10, &newTarget)
	if err != nil {
		t.Fatalf("UpdateCampaignTarget() error = %v", err)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("ClosedAt = %v, want nil", updated.ClosedAt)
	}
	if updated.CloseReason != "" {
		t.Fatalf("CloseReason = %q, want empty", updated.CloseReason)
	}
	if updated.Status != domain.CampaignStatusActive {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.CampaignStatusActive)
	}
}

func TestUpdateCampaignTargetClosesWhenGoalAlreadyReached(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	currentTarget := int64(750000)
	newTarget := int64(500000)
	lastSnapshotAt := now.Add(-3 * time.Hour)
	store := &campaignStoreStub{
		getCampaign: domain.Campaign{
			ID:             10,
			TelegramUserID: 1,
			ChannelID:      "ch",
			Keyword:        "kw",
			StartDate:      now.AddDate(0, 0, -1),
			Timezone:       "UTC",
			TargetViews:    &currentTarget,
			Status:         domain.CampaignStatusActive,
			LastSnapshotAt: &lastSnapshotAt,
			LastTotalViews: 600000,
		},
	}
	statsRepo := &statsRepoStub{}
	stats := stats_service.New(statsRepo, nil)
	service := New(store, stats, nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	updated, err := service.UpdateCampaignTarget(context.Background(), 1, 10, &newTarget)
	if err != nil {
		t.Fatalf("UpdateCampaignTarget() error = %v", err)
	}
	if updated.ClosedAt == nil {
		t.Fatal("ClosedAt = nil, want non-nil")
	}
	if updated.CloseReason != domain.CampaignCloseReasonTargetReached {
		t.Fatalf("CloseReason = %q, want %q", updated.CloseReason, domain.CampaignCloseReasonTargetReached)
	}
	if updated.Status != domain.CampaignStatusClosed {
		t.Fatalf("Status = %q, want %q", updated.Status, domain.CampaignStatusClosed)
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

func TestIsNotificationDueUsesInterval(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	service := New(&campaignStoreStub{}, stats_service.New(&statsRepoStub{}, nil), nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	due, err := service.isNotificationDue(now, domain.UserSettings{
		TelegramUserID:              1,
		NotificationsEnabled:        true,
		NotificationTime:            "12:00",
		NotificationIntervalMinutes: 180,
		LastNotificationSentAt:      ptrTime(now.Add(-3 * time.Hour)),
	})
	if err != nil {
		t.Fatalf("isNotificationDue() error = %v", err)
	}
	if !due {
		t.Fatal("isNotificationDue() = false, want true")
	}
}

func TestIsNotificationDueUsesDailyAnchorTime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 11, 0, 0, 0, time.UTC)
	service := New(&campaignStoreStub{}, stats_service.New(&statsRepoStub{}, nil), nil, nil, "secret", "http://localhost", "state-secret")
	service.now = func() time.Time { return now }

	due, err := service.isNotificationDue(now, domain.UserSettings{
		TelegramUserID:              1,
		NotificationsEnabled:        true,
		NotificationTime:            "12:00",
		NotificationIntervalMinutes: 1440,
		Timezone:                    "UTC",
	})
	if err != nil {
		t.Fatalf("isNotificationDue() error = %v", err)
	}
	if due {
		t.Fatal("isNotificationDue() = true, want false before 12:00")
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func ptrInt64(value int64) *int64 {
	return &value
}

var _ stats_repository.Repository = (*statsRepoStub)(nil)
var _ sponsorblock_repository.Repository = (*sponsorBlockRepoStub)(nil)
