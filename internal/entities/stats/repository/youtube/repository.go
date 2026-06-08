package youtube_repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Repository struct {
	youtube *youtube.Service
}

func New(ctx context.Context, apiKey string) (*Repository, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("youtube api key is empty")
	}

	service, err := youtube.NewService(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("init youtube service: %w", err)
	}

	return &Repository{youtube: service}, nil
}

func (r *Repository) GetVideos(ctx context.Context, query domain.StatsQuery) ([]domain.StatsVideo, error) {
	channelResp, err := r.youtube.Channels.List([]string{"contentDetails"}).Context(ctx).Id(query.ChannelID).Do()
	if err != nil {
		return nil, fmt.Errorf("get channel: %w", err)
	}
	if len(channelResp.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", query.ChannelID)
	}

	uploadPlaylistID := channelResp.Items[0].ContentDetails.RelatedPlaylists.Uploads

	var videos []domain.StatsVideo
	nextPageToken := ""
	done := false

	for !done {
		playlistResp, err := r.youtube.PlaylistItems.List([]string{"contentDetails"}).
			Context(ctx).
			PlaylistId(uploadPlaylistID).
			MaxResults(50).
			PageToken(nextPageToken).
			Do()
		if err != nil {
			return nil, fmt.Errorf("get playlist: %w", err)
		}

		videoIDs := make([]string, 0, len(playlistResp.Items))
		for _, item := range playlistResp.Items {
			videoIDs = append(videoIDs, item.ContentDetails.VideoId)
		}
		if len(videoIDs) == 0 {
			break
		}

		pageVideos, stop, err := r.loadVideosPage(ctx, strings.Join(videoIDs, ","), query)
		if err != nil {
			return nil, err
		}
		videos = append(pageVideos, videos...)
		done = stop

		if playlistResp.NextPageToken == "" {
			break
		}
		nextPageToken = playlistResp.NextPageToken
	}

	if len(query.HiddenVideos) > 0 {
		hiddenVideos, _, err := r.loadVideosPage(ctx, strings.Join(query.HiddenVideos, ","), query)
		if err != nil {
			return nil, err
		}
		videos = append(hiddenVideos, videos...)
	}

	return videos, nil
}

func (r *Repository) loadVideosPage(ctx context.Context, ids string, query domain.StatsQuery) ([]domain.StatsVideo, bool, error) {
	videoResp, err := r.youtube.Videos.List([]string{"snippet", "statistics"}).
		Context(ctx).
		Id(ids).
		Do()
	if err != nil {
		return nil, false, fmt.Errorf("get videos: %w", err)
	}

	var (
		videos []domain.StatsVideo
		stop   bool
	)

	for _, video := range videoResp.Items {
		if video.Id == "" || video.Snippet == nil || video.Statistics == nil {
			continue
		}

		publishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
		if err != nil {
			return nil, false, fmt.Errorf("parse published date for video %s: %w", video.Id, err)
		}

		if publishedAt.Before(query.StartDate) {
			stop = true
			break
		}
		if publishedAt.After(query.EndDate) {
			continue
		}
		if !strings.Contains(video.Snippet.Description, query.AdWord) {
			continue
		}

		videos = append(videos, domain.NewStatsVideo(
			video.Snippet.Title,
			publishedAt,
			video.Statistics.ViewCount,
			fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id),
		))
	}

	return videos, stop, nil
}
