package config

import (
	"os"
	"path/filepath"
	"strings"
)

func ResolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DownloaderDownloadPath
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	path = strings.NewReplacer(
		"{TMP}", os.TempDir(),
		"{TEMP}", os.TempDir(),
		"{PWD}", wd,
		"{CWD}", wd,
	).Replace(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
	}

	return filepath.Clean(path)
}
