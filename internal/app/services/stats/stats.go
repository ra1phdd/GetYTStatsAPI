package auth_v1

import (
	"GetYTStatsAPI/internal/app/models"
	"GetYTStatsAPI/pkg/cache"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	ApiKey string
}

func New(ApiKey string) *Service {
	return &Service{
		ApiKey: ApiKey,
	}
}

func (s Service) GetVideos(channelId string, adWord string, startDate time.Time, maxResults int) ([]models.VideoInfo, error) {
	cacheKey := fmt.Sprintf("videoMap:%s:%s_%s_%d", channelId, adWord, startDate, maxResults)
	cacheData, err := cache.Rdb.Get(cache.Ctx, cacheKey).Result()
	if err == nil {
		var cachedVideoMap []models.VideoInfo
		err := json.Unmarshal([]byte(cacheData), &cachedVideoMap)
		if err != nil {
			return nil, err
		}
		return cachedVideoMap, nil
	}

	ctx := context.Background()
	youtubeService, err := youtube.NewService(ctx, option.WithAPIKey(s.ApiKey))
	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	// Получение видео с канала, начиная с указанной даты
	var videos []models.VideoInfo
	var countViews uint64
	pages := math.Ceil(float64(maxResults) / 50.0)

	// Переменные для пагинации
	nextPageToken := ""
	for i := 0; i < int(pages); i++ {
		var videoIds []string

		call := youtubeService.Search.List([]string{"id"}).
			ChannelId(channelId).
			PublishedAfter(startDate.Format("2006-01-02T15:04:05Z07:00")).
			MaxResults(50).
			Order("date").
			PageToken(nextPageToken)

		response, err := call.Do()
		if err != nil {
			log.Fatalf("Error making API call: %v", err)
		}

		// Обработка результатов
		for _, item := range response.Items {
			if item.Id.Kind == "youtube#video" {
				videoIds = append(videoIds, item.Id.VideoId)
			}
		}

		// Пакетный запрос для получения статистики просмотров
		videoCall := youtubeService.Videos.List([]string{"snippet", "statistics"}).
			Id(strings.Join(videoIds, ","))

		videoResponse, err := videoCall.Do()
		if err != nil {
			log.Fatalf("Error retrieving videos statistics: %v", err)
		}

		// Обработка результатов запроса статистики
		for _, video := range videoResponse.Items {
			if video.Id == "" {
				continue
			}
			videoId := video.Id
			videoTitle := video.Snippet.Title
			videoDescription := video.Snippet.Description
			videoViews := video.Statistics.ViewCount
			videoPublishedAt, err := time.Parse("2006-01-02T15:04:05Z", video.Snippet.PublishedAt)
			if err != nil {
				return nil, err
			}

			// Проверяем наличие ключевого слова в описании
			if strings.Contains(videoDescription, adWord) {
				countViews += videoViews

				videoInfo := models.VideoInfo{
					Name:        videoTitle,
					PublishDate: fmt.Sprint(videoPublishedAt.Format("2006-01-02 15:04")),
					Views:       strconv.FormatUint(videoViews, 10),
					URL:         fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoId),
				}

				// Добавляем видео в начало списка (для обратного порядка)
				videos = append([]models.VideoInfo{videoInfo}, videos...)
			}
		}

		// Проверяем, есть ли следующая страница
		nextPageToken = response.NextPageToken
		if nextPageToken == "" {
			break // Если следующей страницы нет, выходим из цикла
		}
	}

	videos = append(videos, models.VideoInfo{
		Name:  "Общее количество просмотров:",
		Views: strconv.FormatUint(countViews, 10),
	})

	videoMapJSON, err := json.Marshal(videos)
	if err == nil {
		err = cache.Rdb.Set(ctx, cacheKey, videoMapJSON, 5*time.Minute).Err()
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	return videos, nil
}

func (s Service) CreateCSV(data []models.VideoInfo) (*os.File, error) {
	hash, err := hashMap(data)
	if err != nil {
		return nil, err
	}
	// Создаем временный файл
	file, err := os.CreateTemp("", fmt.Sprintf("stats-%s.csv", hash))
	if err != nil {
		return nil, err
	}

	// Создаем CSV writer и записываем данные
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Записываем заголовок CSV
	header := []string{"ID", "Дата", "Название", "Просмотры", "URL"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for id, item := range data {
		var record []string
		if item.PublishDate == "" && item.URL == "" {
			record = []string{
				"",
				"",
				item.Name,
				item.Views,
				"",
			}
		} else {
			record = []string{
				fmt.Sprint(id),
				item.PublishDate,
				item.Name,
				item.Views,
				item.URL,
			}
		}

		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}

	writer.Flush()

	// Закрываем файл перед отправкой его клиенту
	err = file.Close()
	if err != nil {
		return nil, err
	}

	return file, nil
}

func hashMap(data []models.VideoInfo) (string, error) {
	// Сериализуем map в JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling map to JSON: %v", err)
	}

	// Создаем хэш
	hash := sha256.New()
	hash.Write(jsonData)

	// Преобразуем хэш в строку
	hashString := hex.EncodeToString(hash.Sum(nil))

	return hashString, nil
}
