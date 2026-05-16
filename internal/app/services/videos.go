package services

import (
	"context"
	"encoding/csv"
	"fmt"
	"getytstatsapi/internal/app/config"
	"getytstatsapi/internal/app/models"
	"getytstatsapi/pkg/cache"
	"getytstatsapi/pkg/logger"
	"github.com/google/uuid"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Videos struct {
	log   *logger.Logger
	cache *cache.Cache

	youtube *youtube.Service
}

func NewVideos(log *logger.Logger, cfg *config.Configuration, cache *cache.Cache) *Videos {
	yt, err := youtube.NewService(context.Background(), option.WithAPIKey(cfg.YoutubeAPI))
	if err != nil {
		log.Fatal("Failed init youtube service", err)
	}

	return &Videos{
		log:     log,
		cache:   cache,
		youtube: yt,
	}
}

func (vs *Videos) GetVideos(channelId string, adWord string, startDateStr, endDateStr string, hiddenVideos []string) (videos []models.VideoInfo, err error) {
	cacheKey := fmt.Sprintf("videos:%s:%s:%s_%s", channelId, adWord, startDateStr, endDateStr)
	if err = vs.cache.Get(cacheKey, &videos); err == nil {
		vs.log.Debug("Returning servers from cache", slog.String("cache_key", cacheKey))
		return videos, nil
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		vs.log.Error("Error parsing start date", err, slog.String("start_date", startDateStr))
		return nil, err
	}

	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			vs.log.Error("Error parsing end date", err, slog.String("end_date", endDateStr))
			return nil, err
		}
	}

	channelResp, err := vs.youtube.Channels.List([]string{"contentDetails"}).Id(channelId).Do()
	if err != nil {
		vs.log.Error("Error getting channel", err, slog.String("channel_id", channelId))
		return nil, err
	}
	if len(channelResp.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelId)
	}
	uploadPlaylistId := channelResp.Items[0].ContentDetails.RelatedPlaylists.Uploads

	nextPageToken := ""
	done := false
	for !done {
		playlistResp, err := vs.youtube.PlaylistItems.List([]string{"contentDetails"}).
			PlaylistId(uploadPlaylistId).MaxResults(50).PageToken(nextPageToken).Do()
		if err != nil {
			vs.log.Error("Error getting playlist", err, slog.String("playlist_id", uploadPlaylistId))
			return nil, err
		}

		var videoIds []string
		for _, item := range playlistResp.Items {
			videoIds = append(videoIds, item.ContentDetails.VideoId)
		}
		if len(videoIds) == 0 {
			break
		}

		videoResp, err := vs.youtube.Videos.List([]string{"snippet", "statistics"}).
			Id(strings.Join(videoIds, ",")).Do()
		if err != nil {
			vs.log.Error("Failed to get video", err)
			return nil, fmt.Errorf("error retrieving video statistics: %w", err)
		}

		for _, video := range videoResp.Items {
			if video.Id == "" || video.Snippet == nil || video.Statistics == nil {
				continue
			}

			videoPublishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				vs.log.Error("Error parsing published date", err, slog.String("video", video.Id))
				return nil, err
			}

			if videoPublishedAt.Before(startDate) {
				done = true
				break
			}

			if videoPublishedAt.After(endDate) {
				continue
			}

			if strings.Contains(video.Snippet.Description, adWord) {
				videos = append([]models.VideoInfo{{
					Name:        video.Snippet.Title,
					PublishDate: videoPublishedAt.Format("2006-01-02 15:04"),
					Views:       video.Statistics.ViewCount,
					URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id),
				}}, videos...)
			}
		}

		if playlistResp.NextPageToken == "" {
			break
		}
		nextPageToken = playlistResp.NextPageToken
	}

	videoResp, err := vs.youtube.Videos.List([]string{"snippet", "statistics"}).
		Id(strings.Join(hiddenVideos, ",")).Do()
	if err != nil {
		vs.log.Error("Failed to get video", err)
		return nil, fmt.Errorf("error retrieving video statistics: %w", err)
	}

	for _, video := range videoResp.Items {
		if video.Id == "" || video.Snippet == nil || video.Statistics == nil {
			continue
		}

		videoPublishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
		if err != nil {
			vs.log.Error("Error parsing published date", err, slog.String("video", video.Id))
			return nil, err
		}

		if strings.Contains(video.Snippet.Description, adWord) {
			videos = append([]models.VideoInfo{{
				Name:        video.Snippet.Title,
				PublishDate: videoPublishedAt.Format("2006-01-02 15:04"),
				Views:       video.Statistics.ViewCount,
				URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id),
			}}, videos...)
		}
	}

	vs.cache.Set(cacheKey, videos, 5*time.Minute)
	return videos, nil
}

func (vs *Videos) CreateCSV(data []models.VideoInfo) (*os.File, error) {
	file, err := os.CreateTemp("", fmt.Sprintf("stats-%s.csv", uuid.New()))
	if err != nil {
		return nil, err
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"ID", "Дата", "Название", "Просмотры", "URL"}
	if err := writer.Write(header); err != nil {
		vs.log.Error("Error writing csv header", err)
	}

	var countViews uint64
	for id, item := range data {
		record := []string{
			fmt.Sprint(id + 1),
			"'" + item.PublishDate,
			item.Name,
			fmt.Sprint(item.Views),
			item.URL,
		}
		countViews += item.Views

		if err := writer.Write(record); err != nil {
			vs.log.Error("Error writing csv header", err)
		}
	}

	footer := []string{"", "", "Общее количество просмотров:", fmt.Sprint(countViews), ""}
	if err := writer.Write(footer); err != nil {
		vs.log.Error("Error writing csv header", err)
	}

	writer.Flush()
	_ = file.Close()

	return file, nil
}
