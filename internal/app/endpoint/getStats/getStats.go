package getStats

import (
	"GetYTStatsAPI/internal/app/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strconv"
	"time"
)

type GetStats interface {
	GetVideos(channelId string, adWord string, startDate time.Time, maxResults int) ([]models.VideoInfo, error)
	CreateCSV(data []models.VideoInfo) (*os.File, error)
}

type Endpoint struct {
	GetStats GetStats
}

// GetStatsHandler получает статистику видео по заданным параметрам
//
// swagger:route GET /stats/{channel_id}/{ad_word}/{start_date}/{max_results} stats getStatsHandler
//
// Получение статистики видео по идентификатору канала, ключевому слову для рекламы, дате начала и максимальному количеству результатов.
// Этот эндпоинт возвращает CSV-файл со статистикой видео.
//
// Responses:
//
//	200: file
//	500: string
func (e Endpoint) GetStatsHandler(c *gin.Context) {
	channelId := c.Param("channel_id")
	adWord := c.Param("ad_word")
	startDate := c.Param("start_date")
	maxResultsStr := c.Param("max_results")

	date, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}

	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil {
		maxResults = 50
	}

	videos, err := e.GetStats.GetVideos(channelId, adWord, date, maxResults)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}

	file, err := e.GetStats.CreateCSV(videos)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}

	// Настраиваем заголовки для ответа
	c.Header("Content-Disposition", "attachment; filename=stats.csv")
	c.Header("Content-Type", "text/csv")

	// Отправляем CSV-файл в ответе
	c.File(file.Name())
}
