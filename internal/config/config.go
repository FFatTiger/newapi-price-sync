package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type SourceConfig struct {
	Type    string            `yaml:"type"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Timeout time.Duration     `yaml:"timeout"`
	Enabled bool              `yaml:"enabled"`
}

type DatabaseConfig struct {
	Type       string `yaml:"type"`
	DSN        string `yaml:"dsn"`
	SQLitePath string `yaml:"sqlite_path"`
}

type SyncConfig struct {
	Interval            time.Duration `yaml:"interval"`
	Once                bool          `yaml:"once"`
	DryRun              bool          `yaml:"dry_run"`
	PreserveUnmentioned bool          `yaml:"preserve_unmentioned"`
}

type CurrencyConfig struct {
	TargetCurrency  string  `yaml:"target_currency"`
	ExchangeRate    float64 `yaml:"exchange_rate"`
	PriceMultiplier float64 `yaml:"price_multiplier"`
}

type FilterConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	JSON  bool   `yaml:"json"`
}

type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Sources  []SourceConfig `yaml:"sources"`
	Sync     SyncConfig     `yaml:"sync"`
	Currency CurrencyConfig `yaml:"currency"`
	Filter   FilterConfig   `yaml:"filter"`
	Logging  LoggingConfig  `yaml:"logging"`
}

func Default() *Config {
	return &Config{
		Database: DatabaseConfig{
			Type:       "sqlite",
			SQLitePath: "/data/one-api.db",
		},
		Sources: []SourceConfig{
			{
				Type:    "models_dev",
				URL:     "https://models.dev/api.json",
				Timeout: 30 * time.Second,
				Enabled: true,
			},
		},
		Sync: SyncConfig{
			Interval:            24 * time.Hour,
			PreserveUnmentioned: true,
		},
		Currency: CurrencyConfig{
			TargetCurrency:  "USD",
			ExchangeRate:    1,
			PriceMultiplier: 1,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}
	applyEnv(cfg)
	cfg.normalize()
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("SQL_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("SQLITE_PATH"); v != "" {
		cfg.Database.SQLitePath = v
	}
	if v := os.Getenv("NPS_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Sync.Interval = d
		}
	}
	if v := os.Getenv("NPS_EXCHANGE_RATE"); v != "" {
		fmt.Sscanf(v, "%f", &cfg.Currency.ExchangeRate)
	}
	if v := os.Getenv("NPS_PRICE_MULTIPLIER"); v != "" {
		fmt.Sscanf(v, "%f", &cfg.Currency.PriceMultiplier)
	}
	if v := os.Getenv("NPS_TARGET_CURRENCY"); v != "" {
		cfg.Currency.TargetCurrency = v
	}
	if v := os.Getenv("NPS_DRY_RUN"); strings.EqualFold(v, "true") || v == "1" {
		cfg.Sync.DryRun = true
	}
	if v := os.Getenv("NPS_ONCE"); strings.EqualFold(v, "true") || v == "1" {
		cfg.Sync.Once = true
	}
}

func (c *Config) normalize() {
	if c.Sync.Interval <= 0 {
		c.Sync.Interval = 24 * time.Hour
	}
	if c.Currency.ExchangeRate == 0 {
		c.Currency.ExchangeRate = 1
	}
	if c.Currency.PriceMultiplier == 0 {
		c.Currency.PriceMultiplier = 1
	}
	if c.Database.SQLitePath == "" {
		c.Database.SQLitePath = "/data/one-api.db"
	}
	if c.Database.DSN == "" || strings.HasPrefix(c.Database.DSN, "local") {
		c.Database.Type = "sqlite"
	} else if strings.HasPrefix(c.Database.DSN, "postgres://") || strings.HasPrefix(c.Database.DSN, "postgresql://") {
		c.Database.Type = "postgres"
	} else {
		c.Database.Type = "mysql"
	}
	for i := range c.Sources {
		if c.Sources[i].Timeout <= 0 {
			c.Sources[i].Timeout = 30 * time.Second
		}
		if !c.Sources[i].Enabled {
			continue
		}
	}
}

func (c *Config) EffectivePriceMultiplier() float64 {
	return c.Currency.ExchangeRate * c.Currency.PriceMultiplier
}
