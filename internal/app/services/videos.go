package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"getytstatsapi/internal/app/config"
	"getytstatsapi/internal/app/models"
	"getytstatsapi/pkg/cache"
	"getytstatsapi/pkg/logger"
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

func (vs *Videos) GetVideos(channelId string, adWord string, startDateStr, endDateStr string) (videos []models.VideoInfo, err error) {
	cacheKey := fmt.Sprintf("videos:%s:%s:%s_%s", channelId, adWord, startDateStr, endDateStr)
	if err = vs.cache.Get(cacheKey, &videos); err == nil {
		vs.log.Debug("Returning servers from cache", slog.String("cache_key", cacheKey))
		return videos, nil
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, err
	}

	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, err
		}
	}

	var videoIds []string
	nextPageToken := ""

	for {
		call := vs.youtube.Search.List([]string{"id"}).
			ChannelId(channelId).
			PublishedAfter(startDate.Format(time.RFC3339)).
			PublishedBefore(endDate.Format(time.RFC3339)).
			Order("date").
			Type("video").
			MaxResults(50)

		if nextPageToken != "" {
			call = call.PageToken(nextPageToken)
		}

		response, err := call.Do()
		if err != nil {
			return nil, err
		}

		for _, item := range response.Items {
			videoIds = append(videoIds, item.Id.VideoId)
		}

		if response.NextPageToken == "" {
			break
		}
		nextPageToken = response.NextPageToken
	}
	vs.log.Debug("Total videos found", slog.Int("count", len(videoIds)))

	for i := 0; i < len(videoIds); i += 50 {
		end := i + 50
		if end > len(videoIds) {
			end = len(videoIds)
		}
		idsBatch := videoIds[i:end]

		videoCall := vs.youtube.Videos.List([]string{"snippet", "statistics"}).
			Id(strings.Join(idsBatch, ",")).
			Fields(
				"items(" +
					"etag,id,kind," +
					"snippet(publishedAt,title,description,thumbnails/default)," +
					"statistics)",
			)

		videoResponse, err := videoCall.Do()
		if err != nil {
			return nil, fmt.Errorf("error retrieving video statistics: %w", err)
		}

		for _, video := range videoResponse.Items {
			if video.Id == "" {
				continue
			}
			videoViews := video.Statistics.ViewCount
			videoPublishedAt, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return nil, err
			}
			if strings.Contains(video.Snippet.Description, adWord) {
				videos = append([]models.VideoInfo{{
					Name:        video.Snippet.Title,
					PublishDate: videoPublishedAt.Format("2006-01-02 15:04"),
					Views:       videoViews,
					URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id),
				}}, videos...)
			}
		}
	}

	videoMapJSON, err := json.Marshal(videos)
	if err == nil {
		vs.cache.Set(cacheKey, videoMapJSON, 5*time.Minute)
	} else {
		return nil, err
	}

	return videos, nil
}

func (vs *Videos) CreateCSV(data []models.VideoInfo) (*os.File, error) {
	hash, err := hashMap(data)
	if err != nil {
		return nil, err
	}

	file, err := os.CreateTemp("", fmt.Sprintf("stats-%s.csv", hash))
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
