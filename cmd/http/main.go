package main

import (
	"fmt"
	core_config "getytstatsapi/internal/core/infra/config"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	base_config "github.com/ra1phdd/config"
	"github.com/ra1phdd/logger"
)

func main() {
	loader, err := core_config.NewLoader(core_config.HTTPConfig)
	if err != nil {
		panic(err)
	}

	cfg, err := loader.Load()
	if err != nil {
		panic(err)
	}

	webRoot := base_config.ResolvePath(cfg.Web.Root)
	indexPath := filepath.Join(webRoot, "index.html")
	if info, err := os.Stat(webRoot); err != nil || !info.IsDir() {
		panic(fmt.Errorf("web directory %q is not available", webRoot))
	}
	if _, err := os.Stat(indexPath); err != nil {
		panic(fmt.Errorf("web entrypoint %q is not available: %w", indexPath, err))
	}

	log := logger.New(
		logger.WithComponent("http"),
		logger.WithLevelString(cfg.LoggerLevel),
	)

	fileServer := http.FileServer(http.Dir(webRoot))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		relPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if relPath != "" {
			filePath := filepath.Join(webRoot, filepath.FromSlash(relPath))
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		http.ServeFile(w, r, indexPath)
	})

	log.Info(
		"start static HTTP server",
		logger.String("addr", cfg.HTTP.Address),
		logger.String("root", webRoot),
	)

	if err := http.ListenAndServe(cfg.HTTP.Address, mux); err != nil {
		panic(err)
	}
}
