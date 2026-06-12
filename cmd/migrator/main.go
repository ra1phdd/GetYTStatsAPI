package main

import (
	"flag"
	"fmt"
	core_config "getytstatsapi/internal/core/infra/config"
	"getytstatsapi/internal/core/infra/postgres"
	"path/filepath"
	"strings"

	"github.com/pressly/goose/v3"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	dir := flag.String("dir", "migrations", "path to migrations directory")
	flag.Parse()

	loader, err := core_config.NewLoader(core_config.MainConfig)
	if err != nil {
		panic(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		panic(err)
	}

	db, err := postgres.Open(cfg.Database.DSN())
	if err != nil {
		panic(err)
	}
	defer db.Close()

	migrationsDir, err := filepath.Abs(strings.TrimSpace(*dir))
	if err != nil {
		panic(err)
	}

	if err := goose.SetDialect("postgres"); err != nil {
		panic(err)
	}

	switch strings.ToLower(strings.TrimSpace(*direction)) {
	case "up":
		err = goose.Up(db, migrationsDir)
	case "down":
		err = goose.Down(db, migrationsDir)
	default:
		err = fmt.Errorf("unsupported direction %q, use up or down", *direction)
	}
	if err != nil {
		panic(err)
	}
}
