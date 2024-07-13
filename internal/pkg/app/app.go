package app

import (
	"GetYTStatsAPI/config"
	"GetYTStatsAPI/internal/app/endpoint/getStats"
	stats "GetYTStatsAPI/internal/app/services/stats"
	"GetYTStatsAPI/pkg/cache"
	"GetYTStatsAPI/pkg/logger"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type App struct {
	getStats *stats.Service

	router *gin.Engine
}

func New(cfg *config.Configuration) (*App, error) {
	gin.SetMode(cfg.GinMode)

	logger.Init(cfg.LoggerLevel)

	a := &App{}

	a.router = gin.Default()

	// регистрируем сервисы
	a.getStats = stats.New(cfg.ApiKey)

	// регистрируем эндпоинты
	serviceStats := &getStats.Endpoint{
		GetStats: a.getStats,
	}

	// регистрируем маршруты
	a.router.GET("/get_stats/:channel_id/:ad_word/:start_date/:max_results", serviceStats.GetStatsHandler)

	err := cache.Init(cfg.Redis.RedisAddr+":"+cfg.Redis.RedisPort, cfg.Redis.RedisUsername, cfg.Redis.RedisPassword, cfg.Redis.RedisDBId)
	if err != nil {
		logger.Error("ошибка при инициализации кэша: ", zap.Error(err))
		return nil, err
	}

	return a, nil
}

func (a *App) Run(port string) error {
	err := a.router.Run(fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}

	return nil
}
