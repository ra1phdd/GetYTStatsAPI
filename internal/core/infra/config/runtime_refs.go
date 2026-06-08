package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileScheme = "file://"

func LoadRuntimeRefs(cfg *Config, configPath string) error {
	if cfg == nil {
		return nil
	}
	if cfg.runtimeRefs.ChatUsers != "" {
		var data ChatUsersConfig
		if err := readRuntimeRef(configPath, cfg.runtimeRefs.ChatUsers, &data); err != nil {
			return fmt.Errorf("load chat_users: %w", err)
		}
		cfg.ChatUsers = data
	}
	return nil
}

func SaveRuntimeRefs(cfg *Config, configPath string) error {
	if cfg == nil {
		return nil
	}
	if cfg.runtimeRefs.ChatUsers != "" {
		if err := writeRuntimeRef(configPath, cfg.runtimeRefs.ChatUsers, cfg.ChatUsers); err != nil {
			return fmt.Errorf("save chat_users: %w", err)
		}
	}
	return nil
}

func readRuntimeRef(configPath string, ref string, target any) error {
	path, err := runtimeRefPath(configPath, ref)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func writeRuntimeRef(configPath string, ref string, value any) error {
	path, err := runtimeRefPath(configPath, ref)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func runtimeRefPath(configPath string, ref string) (string, error) {
	fileName := strings.TrimSpace(strings.TrimPrefix(ref, fileScheme))
	if fileName == "" || strings.HasPrefix(fileName, fileScheme) {
		return "", fmt.Errorf("invalid file reference %q", ref)
	}
	if filepath.IsAbs(fileName) {
		return filepath.Clean(fileName), nil
	}
	return filepath.Clean(filepath.Join(filepath.Dir(configPath), fileName)), nil
}
