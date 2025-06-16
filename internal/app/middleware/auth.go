package middleware

import (
	"getytstatsapi/internal/app/services"
	"getytstatsapi/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Auth struct {
	log   *logger.Logger
	servs *services.Services
}

func NewAuth(log *logger.Logger, servs *services.Services) *Auth {
	return &Auth{
		log:   log,
		servs: servs,
	}
}

func (a *Auth) IsAuth(c *gin.Context) {
	c.Next()
}
