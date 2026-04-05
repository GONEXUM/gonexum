package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// configFilePath is optionally overridden via --config flag
var configFilePath string

func settingsPath() (string, error) {
	if configFilePath != "" {
		return configFilePath, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(configDir, "GONEXUM")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

func loadSettings() (Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return Settings{TrackerURL: "https://nexum-core.com"}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{TrackerURL: "https://nexum-core.com"}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	if s.TrackerURL == "" {
		s.TrackerURL = "https://nexum-core.com"
	}
	return s, nil
}

func saveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
