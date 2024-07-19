package getStats

import (
	"GetYTStatsAPI/internal/app/models"
	"fmt"
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

func (e Endpoint) GetCommandHandler(c *gin.Context) {
	channelId := c.Query("channel_id")
	adWord := c.Query("ad_word")
	startDate := c.Query("start_date")
	maxResultsStr := c.Query("max_results")

	c.String(http.StatusOK, fmt.Sprintf("=IMPORTDATA(\"http://91.215.21.236:8089/get_stats?channel_id=%s&ad_word=%s&start_date=%s&max_results=%s\";\",\";\"en_US\")", channelId, adWord, startDate, maxResultsStr))
}

func (e Endpoint) GetStatsHandler(c *gin.Context) {
	channelId := c.Query("channel_id")
	adWord := c.Query("ad_word")
	startDate := c.Query("start_date")
	maxResultsStr := c.Query("max_results")

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
