package domain

import "time"

type StatsQuery struct {
	ChannelID    string
	AdWord       string
	StartDate    time.Time
	EndDate      time.Time
	HiddenVideos []string
}

func NewStatsQuery(
	channelID string,
	adWord string,
	startDate time.Time,
	endDate time.Time,
	hiddenVideos []string,
) StatsQuery {
	return StatsQuery{
		ChannelID:    channelID,
		AdWord:       adWord,
		StartDate:    startDate,
		EndDate:      endDate,
		HiddenVideos: hiddenVideos,
	}
}

type StatsVideo struct {
	Name        string
	PublishDate time.Time
	Views       uint64
	URL         string
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
