package domain

import "time"

type StatsColumn string

const (
	StatsColumnID             StatsColumn = "id"
	StatsColumnPublishDate    StatsColumn = "publish_date"
	StatsColumnVideoURL       StatsColumn = "video_url"
	StatsColumnViews          StatsColumn = "views"
	StatsColumnAdTimings      StatsColumn = "ad_timings"
	StatsColumnViewsUpdatedAt StatsColumn = "views_updated_at"
)

type StatsQuery struct {
	ChannelID    string
	AdWord       string
	StartDate    time.Time
	EndDate      time.Time
	HiddenVideos []string
	Columns      []StatsColumn
}

func NewStatsQuery(
	channelID string,
	adWord string,
	startDate time.Time,
	endDate time.Time,
	hiddenVideos []string,
	columns []StatsColumn,
) StatsQuery {
	return StatsQuery{
		ChannelID:    channelID,
		AdWord:       adWord,
		StartDate:    startDate,
		EndDate:      endDate,
		HiddenVideos: hiddenVideos,
		Columns:      normalizeStatsColumns(columns),
	}
}

func DefaultStatsColumns() []StatsColumn {
	return []StatsColumn{
		StatsColumnID,
		StatsColumnPublishDate,
		StatsColumnVideoURL,
		StatsColumnViews,
	}
}

func NormalizeStatsColumns(columns []StatsColumn) []StatsColumn {
	return normalizeStatsColumns(columns)
}

func IsValidStatsColumn(column StatsColumn) bool {
	switch column {
	case StatsColumnID,
		StatsColumnPublishDate,
		StatsColumnVideoURL,
		StatsColumnViews,
		StatsColumnAdTimings,
		StatsColumnViewsUpdatedAt:
		return true
	default:
		return false
	}
}

func normalizeStatsColumns(columns []StatsColumn) []StatsColumn {
	if len(columns) == 0 {
		return DefaultStatsColumns()
	}

	result := make([]StatsColumn, 0, len(columns))
	seen := make(map[StatsColumn]struct{}, len(columns))
	for _, column := range columns {
		if !IsValidStatsColumn(column) {
			continue
		}
		if _, ok := seen[column]; ok {
			continue
		}
		seen[column] = struct{}{}
		result = append(result, column)
	}

	if len(result) == 0 {
		return DefaultStatsColumns()
	}

	return result
}

type StatsVideo struct {
	VideoID        string
	Name           string
	PublishDate    time.Time
	Views          uint64
	URL            string
	ViewsUpdatedAt time.Time
	SkipSegments   []SponsorBlockSegment
}

func NewStatsVideo(
	name string,
	publishDate time.Time,
	views uint64,
	url string,
) StatsVideo {
	return StatsVideo{
		Name:        name,
		PublishDate: publishDate,
		Views:       views,
		URL:         url,
	}
}
