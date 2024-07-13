package config

import (
	"log"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Configuration struct {
	Port        string `env:"PORT" envDefault:"8080"`
	LoggerLevel string `env:"LOGGER_LEVEL" envDefault:"warn"`
	GinMode     string `env:"GIN_MODE" envDefault:"release"`
	ApiKey      string `env:"API_KEY,required"`
	Redis       Redis
}

type Redis struct {
	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPort     string `env:"REDIS_PORT" envDefault:"6379"`
	RedisUsername string `env:"REDIS_USERNAME,required"`
	RedisPassword string `env:"REDIS_PASSWORD,required"`
	RedisDBId     int    `env:"REDIS_DB_ID,required"`
}

func NewConfig(files ...string) (*Configuration, error) {
	err := godotenv.Load(files...)
	if err != nil {
		log.Fatalf("Файл .env не найден: %s", err)
	}

	var cfg Configuration
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	if err := env.Parse(&cfg.Redis); err != nil {
		return nil, err
	}

	return &cfg, nil
}
