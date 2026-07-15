package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Palworld PalworldConfig `yaml:"palworld"`
	Cache    CacheConfig    `yaml:"cache"`
	Database DatabaseConfig `yaml:"database"`
	App      AppConfig      `yaml:"app"`
	Logging  LoggingConfig  `yaml:"logging"`
	Auth     AuthConfig     `yaml:"auth"`
}

type AuthConfig struct {
	Enabled        bool          `yaml:"enabled"`
	PublicBaseURL  string        `yaml:"public_base_url"`
	SessionTTL     time.Duration `yaml:"-"`
	SessionTTLText string        `yaml:"session_ttl"`
	AdminSteamIDs  []string      `yaml:"admin_steam_ids"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}
type PalworldConfig struct {
	BaseURL     string        `yaml:"base_url"`
	Username    string        `yaml:"username"`
	Password    string        `yaml:"password"`
	Timeout     time.Duration `yaml:"-"`
	TimeoutText string        `yaml:"timeout"`
}
type CacheConfig struct {
	InfoTTL        time.Duration `yaml:"-"`
	MetricsTTL     time.Duration `yaml:"-"`
	PlayersTTL     time.Duration `yaml:"-"`
	InfoTTLText    string        `yaml:"info_ttl"`
	MetricsTTLText string        `yaml:"metrics_ttl"`
	PlayersTTLText string        `yaml:"players_ttl"`
}
type AppConfig struct {
	MockMode bool `yaml:"mock_mode"`
}
type DatabaseConfig struct {
	Path string `yaml:"path"`
}
type LoggingConfig struct {
	Level string `yaml:"level"`
}

func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is required (use --config)")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:8091"
	}
	if cfg.Palworld.BaseURL == "" {
		cfg.Palworld.BaseURL = "http://127.0.0.1:8212"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "/var/lib/palworld-companion/companion.db"
	}
	if cfg.Palworld.Timeout, err = durationOrDefault(cfg.Palworld.TimeoutText, 3*time.Second, "palworld.timeout"); err != nil {
		return Config{}, err
	}
	if cfg.Cache.InfoTTL, err = durationOrDefault(cfg.Cache.InfoTTLText, 30*time.Second, "cache.info_ttl"); err != nil {
		return Config{}, err
	}
	if cfg.Cache.MetricsTTL, err = durationOrDefault(cfg.Cache.MetricsTTLText, 5*time.Second, "cache.metrics_ttl"); err != nil {
		return Config{}, err
	}
	if cfg.Cache.PlayersTTL, err = durationOrDefault(cfg.Cache.PlayersTTLText, 3*time.Second, "cache.players_ttl"); err != nil {
		return Config{}, err
	}
	if cfg.Auth.SessionTTL, err = durationOrDefault(cfg.Auth.SessionTTLText, 30*24*time.Hour, "auth.session_ttl"); err != nil {
		return Config{}, err
	}
	if cfg.Auth.Enabled {
		if cfg.Auth.PublicBaseURL == "" {
			return Config{}, errors.New("auth.public_base_url is required when auth is enabled")
		}
		u, parseErr := url.Parse(cfg.Auth.PublicBaseURL)
		if parseErr != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Config{}, errors.New("auth.public_base_url must be an absolute HTTPS URL")
		}
		cfg.Auth.PublicBaseURL = u.String()
		if cfg.Auth.PublicBaseURL[len(cfg.Auth.PublicBaseURL)-1] != '/' {
			cfg.Auth.PublicBaseURL += "/"
		}
	}
	return cfg, nil
}

func durationOrDefault(raw string, fallback time.Duration, name string) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid %s: must be positive", name)
	}
	return d, nil
}
