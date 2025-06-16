package middleware

import (
	"getytstatsapi/pkg/logger"
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

type Logging struct {
	log *logger.Logger
}

func NewLogging(log *logger.Logger) *Logging {
	return &Logging{
		log: log,
	}
}

func (l *Logging) LoggingAPI(c *gin.Context) {
	start := time.Now()
	c.Next()
	latency := time.Since(start)

	l.log.Debug("Calling route API",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.Int("status", c.Writer.Status()),
		slog.String("latency", latency.String()),
	)
}
