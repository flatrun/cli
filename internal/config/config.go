package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	EnvConfig  = "FLATRUN_CONFIG"
	EnvProfile = "FLATRUN_PROFILE"
	EnvURL     = "FLATRUN_URL"
	EnvToken   = "FLATRUN_TOKEN"
)

type Config struct {
	CurrentProfile string             `json:"current_profile"`
	Profiles       map[string]Profile `json:"profiles"`
}

type Profile struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func DefaultPath() string {
	if path := os.Getenv(EnvConfig); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".flatrun/config.json"
	}
	return filepath.Join(home, ".flatrun", "config.json")
}

func Load(path string) (Config, error) {
	cfg := Config{
		CurrentProfile: "default",
		Profiles:       map[string]Profile{},
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.CurrentProfile == "" && len(cfg.Profiles) > 0 {
		cfg.CurrentProfile = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.CurrentProfile == "" && len(cfg.Profiles) > 0 {
		cfg.CurrentProfile = "default"
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]Profile{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0600)
}

func Resolve(profileName string) (Profile, string, error) {
	if profileName == "" {
		profileName = os.Getenv(EnvProfile)
	}

	cfg, err := Load(DefaultPath())
	if err != nil {
		return Profile{}, "", err
	}

	if profileName == "" {
		profileName = cfg.CurrentProfile
	}
	if profileName == "" && len(cfg.Profiles) == 0 {
		return Profile{}, "", fmt.Errorf("no active FlatRun profile; run `flatrun configure set --url URL --token TOKEN` or set %s and %s", EnvURL, EnvToken)
	}
	if profileName == "" {
		profileName = "default"
	}

	profile := cfg.Profiles[profileName]
	if url := os.Getenv(EnvURL); url != "" {
		profile.URL = url
	}
	if token := os.Getenv(EnvToken); token != "" {
		profile.Token = token
	}

	if profile.URL == "" {
		return profile, profileName, fmt.Errorf("missing FlatRun URL; run `flatrun configure set --url URL --token TOKEN` or set %s", EnvURL)
	}
	if profile.Token == "" {
		return profile, profileName, fmt.Errorf("missing FlatRun token; run `flatrun configure set --url URL --token TOKEN` or set %s", EnvToken)
	}
	return profile, profileName, nil
}
