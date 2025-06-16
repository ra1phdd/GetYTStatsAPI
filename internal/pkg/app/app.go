package app

import (
	"context"
	"fmt"
	"getytstatsapi/internal/app/config"
	"getytstatsapi/internal/app/handlers"
	"getytstatsapi/internal/app/middleware"
	"getytstatsapi/internal/app/services"
	"getytstatsapi/pkg/cache"
	"getytstatsapi/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"net/http"
	"time"

	_ "net/http/pprof"
)

type App struct {
	cfg   *config.Configuration
	log   *logger.Logger
	db    *gorm.DB
	cache *cache.Cache

	services    services.Services
	middlewares middleware.Middlewares
	handlers    handlers.Handlers
}

func New() error {
	a := &App{
		log: logger.New(),
	}

	if err := a.initConfig(); err != nil {
		return err
	}

	if err := a.initCache(); err != nil {
		return err
	}

	a.initServices()
	a.initHandlers()
	a.initMiddlewares()

	return a.runServer()
}

func (a *App) initConfig() (err error) {
	a.cfg, err = config.NewConfig()
	if err != nil {
		a.log.Error("Error loading config from env", err)
		return err
	}
	a.log.SetLogLevel(a.cfg.LoggerLevel)
	return nil
}

func (a *App) initCache() error {
	ctx := context.Background()
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", a.cfg.Redis.Address, a.cfg.Redis.Port),
		Username: a.cfg.Redis.Username,
		Password: a.cfg.Redis.Password,
		DB:       a.cfg.Redis.DB,
	})
	err := client.Ping(ctx).Err()
	if err != nil {
		return err
	}

	a.cache = cache.New(a.log, client)
	return nil
}

func (a *App) initServices() {
	a.services.Videos = services.NewVideos(a.log, a.cfg, a.cache)
}

func (a *App) initHandlers() {
	a.handlers.Stats = handlers.NewStats(a.log, a.cfg, &a.services)
}

func (a *App) initMiddlewares() {
	a.middlewares.Auth = middleware.NewAuth(a.log, &a.services)
	a.middlewares.Logging = middleware.NewLogging(a.log)
}

func (a *App) newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func (a *App) runServer() error {
	gin.SetMode(a.cfg.GinMode)

	r := gin.Default()
	r.Use(a.middlewares.Logging.LoggingAPI)

	r.GET("/v1/command/get", a.handlers.Stats.GetCommandHandler)
	r.GET("/v1/stats/get", a.handlers.Stats.GetStatsHandler)

	a.log.Info("Server is running")
	return r.Run(fmt.Sprintf(":" + a.cfg.Port))
}
