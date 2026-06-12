package youtube_repository

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Repository struct {
	youtube *youtube.Service
}

type Channel struct {
	ID          string
	Title       string
	Description string
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
	fetchedAt := time.Now().UTC()

	channel, uploadPlaylistID, err := r.getChannel(ctx, query.ChannelID)
	if err != nil {
		return nil, err
	}
	_ = channel

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

		pageVideos, stop, err := r.loadVideosPage(ctx, strings.Join(videoIDs, ","), query, fetchedAt)
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
		hiddenVideos, _, err := r.loadVideosPage(ctx, strings.Join(query.HiddenVideos, ","), query, fetchedAt)
		if err != nil {
			return nil, err
		}
		videos = append(hiddenVideos, videos...)
	}

	return videos, nil
}

func (r *Repository) GetChannel(ctx context.Context, channelID string) (Channel, error) {
	channel, _, err := r.getChannel(ctx, channelID)
	return channel, err
}

func (r *Repository) getChannel(ctx context.Context, channelID string) (Channel, string, error) {
	resolvedID, err := r.resolveChannelID(ctx, channelID)
	if err != nil {
		return Channel{}, "", err
	}

	channelResp, err := r.youtube.Channels.List([]string{"contentDetails", "snippet"}).Context(ctx).Id(resolvedID).Do()
	if err != nil {
		return Channel{}, "", fmt.Errorf("get channel: %w", err)
	}
	if len(channelResp.Items) == 0 || channelResp.Items[0].ContentDetails == nil || channelResp.Items[0].Snippet == nil {
		return Channel{}, "", fmt.Errorf("channel not found: %s", resolvedID)
	}

	item := channelResp.Items[0]
	return Channel{ID: item.Id, Title: item.Snippet.Title, Description: item.Snippet.Description}, item.ContentDetails.RelatedPlaylists.Uploads, nil
}

func (r *Repository) resolveChannelID(ctx context.Context, value string) (string, error) {
	ref, err := parseChannelReference(value)
	if err != nil {
		return "", err
	}

	switch ref.kind {
	case channelReferenceID:
		return ref.value, nil
	case channelReferenceHandle:
		return r.lookupChannelIDByHandle(ctx, ref.value)
	case channelReferenceUsername:
		return r.lookupChannelIDByUsername(ctx, ref.value)
	default:
		return "", fmt.Errorf("unsupported channel reference")
	}
}

func (r *Repository) lookupChannelIDByHandle(ctx context.Context, handle string) (string, error) {
	handle = strings.TrimSpace(strings.TrimPrefix(handle, "@"))
	if handle == "" {
		return "", fmt.Errorf("channel handle is empty")
	}

	channelResp, err := r.youtube.Channels.List([]string{"id"}).Context(ctx).ForHandle(handle).Do()
	if err != nil {
		return "", fmt.Errorf("get channel by handle: %w", err)
	}
	if len(channelResp.Items) == 0 || channelResp.Items[0].Id == "" {
		return "", fmt.Errorf("channel not found: @%s", handle)
	}
	return channelResp.Items[0].Id, nil
}

func (r *Repository) lookupChannelIDByUsername(ctx context.Context, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", fmt.Errorf("channel username is empty")
	}

	channelResp, err := r.youtube.Channels.List([]string{"id"}).Context(ctx).ForUsername(username).Do()
	if err != nil {
		return "", fmt.Errorf("get channel by username: %w", err)
	}
	if len(channelResp.Items) == 0 || channelResp.Items[0].Id == "" {
		return "", fmt.Errorf("channel not found: %s", username)
	}
	return channelResp.Items[0].Id, nil
}

type channelReferenceKind string

const (
	channelReferenceID       channelReferenceKind = "id"
	channelReferenceHandle   channelReferenceKind = "handle"
	channelReferenceUsername channelReferenceKind = "username"
)

type channelReference struct {
	kind  channelReferenceKind
	value string
}

func parseChannelReference(raw string) (channelReference, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return channelReference{}, fmt.Errorf("channel reference is empty")
	}

	if strings.HasPrefix(raw, "@") {
		return channelReference{kind: channelReferenceHandle, value: strings.TrimPrefix(raw, "@")}, nil
	}

	if strings.HasPrefix(raw, "UC") && !strings.Contains(raw, "/") {
		return channelReference{kind: channelReferenceID, value: raw}, nil
	}

	if !strings.Contains(raw, "://") && strings.Contains(raw, "youtube.com/") {
		raw = "https://" + raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return channelReference{kind: channelReferenceID, value: raw}, nil
	}

	host := strings.ToLower(parsed.Host)
	segments := splitURLPath(parsed.Path)
	if len(segments) == 0 {
		return channelReference{}, fmt.Errorf("unsupported channel url")
	}

	switch host {
	case "www.youtube.com", "youtube.com", "m.youtube.com", "music.youtube.com", "studio.youtube.com":
		if segments[0] == "channel" && len(segments) >= 2 {
			return channelReference{kind: channelReferenceID, value: segments[1]}, nil
		}
		if strings.HasPrefix(segments[0], "@") {
			return channelReference{kind: channelReferenceHandle, value: strings.TrimPrefix(segments[0], "@")}, nil
		}
		if segments[0] == "user" && len(segments) >= 2 {
			return channelReference{kind: channelReferenceUsername, value: segments[1]}, nil
		}
	}

	return channelReference{}, fmt.Errorf("unsupported channel url")
}

func splitURLPath(value string) []string {
	value = strings.TrimSpace(path.Clean("/" + value))
	value = strings.TrimPrefix(value, "/")
	if value == "" || value == "." {
		return nil
	}
	parts := strings.Split(value, "/")
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		filtered = append(filtered, part)
	}
	return filtered
}

func (r *Repository) loadVideosPage(ctx context.Context, ids string, query domain.StatsQuery, fetchedAt time.Time) ([]domain.StatsVideo, bool, error) {
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

		statsVideo := domain.NewStatsVideo(
			video.Snippet.Title,
			publishedAt,
			video.Statistics.ViewCount,
			fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id),
		)
		statsVideo.VideoID = video.Id
		statsVideo.ViewsUpdatedAt = fetchedAt

		videos = append(videos, statsVideo)
	}

	return videos, stop, nil
}
