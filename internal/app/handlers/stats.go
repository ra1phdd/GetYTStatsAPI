package handlers

import (
	"fmt"
	"getytstatsapi/internal/app/config"
	"getytstatsapi/internal/app/services"
	"getytstatsapi/pkg/logger"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Stats struct {
	log   *logger.Logger
	cfg   *config.Configuration
	servs *services.Services
}

func NewStats(log *logger.Logger, cfg *config.Configuration, servs *services.Services) *Stats {
	return &Stats{
		log:   log,
		cfg:   cfg,
		servs: servs,
	}
}

func (s *Stats) GetCommandHandler(c *gin.Context) {
	channelId := c.Query("channel_id")
	adWord := c.Query("ad_word")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	c.String(http.StatusOK, fmt.Sprintf("=IMPORTDATA(\"http://%s:%s/v1/stats/get?channel_id=%s&ad_word=%s&start_date=%s&end_date=%s\";\",\";\"en_US\")", s.cfg.ExternalHost, s.cfg.Port, channelId, adWord, startDate, endDate))
}

func (s *Stats) GetStatsHandler(c *gin.Context) {
	channelId := c.Query("channel_id")
	adWord := c.Query("ad_word")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	videos, err := s.servs.Videos.GetVideos(channelId, adWord, startDate, endDate)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}

	file, err := s.servs.Videos.CreateCSV(videos)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
	}

	if file == nil {
		c.JSON(http.StatusBadRequest, gin.H{})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=stats.csv")
	c.Header("Content-Type", "text/csv")

	c.File(file.Name())
}
