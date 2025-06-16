package config

import (
	"log"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Configuration struct {
	LoggerLevel  string `env:"LOGGER_LEVEL" envDefault:"warn"`
	GinMode      string `env:"GIN_MODE" envDefault:"release"`
	ExternalHost string `env:"EXTERNAL_HOST,required"`
	Port         string `env:"PORT" envDefault:"8080"`
	YoutubeAPI   string `env:"YOUTUBE_API,required"`

	Redis Redis
}

type Redis struct {
	Address  string `env:"REDIS_ADDR,required"`
	Port     int    `env:"REDIS_PORT" envDefault:"6379"`
	Username string `env:"REDIS_USER,required"`
	Password string `env:"REDIS_PASS,required"`
	DB       int    `env:"REDIS_DB,required"`
}

func NewConfig(files ...string) (*Configuration, error) {
	err := godotenv.Load(files...)
	if err != nil {
		log.Fatal("Файл .env не найден", err.Error())
	}

	cfg := Configuration{}
	err = env.Parse(&cfg)
	if err != nil {
		return nil, err
	}
	err = env.Parse(&cfg.Redis)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
