package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server listening and timeout settings.
type ServerConfig struct {
	Port                int    `yaml:"port"`
	Host                string `yaml:"host"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

// SearXNGConfig holds SearXNG meta-search provider settings.
type SearXNGConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Port    int    `yaml:"port"`
}

// SQLiteConfig holds file-based SQLite database settings.
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	URL      string `yaml:"url"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// DatabaseConfig holds primary database configuration (SQLite or PostgreSQL).
type DatabaseConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Type     string         `yaml:"type"` // "sqlite" or "postgres" / "pg"
	SQLite   SQLiteConfig   `yaml:"sqlite"`
	Postgres PostgresConfig `yaml:"postgres"`
}

// RedisConfig holds distributed cache settings.
type RedisConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}

// LogsConfig defines system logging configuration, storing in SQLite by default.
type LogsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Storage       string `yaml:"storage"` // "sqlite", "stdout", "both"
	SQLitePath    string `yaml:"sqlite_path"`
	Level         string `yaml:"level"` // "debug", "info", "warn", "error"
	RetentionDays int    `yaml:"retention_days"`
}

// CacheConfig holds cache TTL settings.
type CacheConfig struct {
	SearchTTLHours int `yaml:"search_ttl_hours"`
	ScrapeTTLDays  int `yaml:"scrape_ttl_days"`
}

// OllamaConfig holds local LLM server settings.
type OllamaConfig struct {
	Enabled        bool   `yaml:"enabled"`
	URL            string `yaml:"url"`
	Model          string `yaml:"model"`
	EmbeddingModel string `yaml:"embedding_model"`
}

// BrowserEngineConfig holds path and enable flags for headless browsers.
type BrowserEngineConfig struct {
	Enabled    bool   `yaml:"enabled"`
	BinaryPath string `yaml:"binary_path"`
}

// ScraperConfig holds rate limits, concurrency, and headless browser options.
type ScraperConfig struct {
	RateLimitPerDomainRPS int                 `yaml:"rate_limit_per_domain_rps"`
	MaxConcurrentScrapes  int                 `yaml:"max_concurrent_scrapes"`
	Lightpanda            BrowserEngineConfig `yaml:"lightpanda"`
	Camoufox              BrowserEngineConfig `yaml:"camoufox"`
}

// ProxiesConfig holds rotating and residential proxy lists.
type ProxiesConfig struct {
	Rotating           []string `yaml:"rotating"`
	ResidentialURL     string   `yaml:"residential_url"`
	ResidentialProxies []string `yaml:"residential_proxies"`
}

// Config is the unified root configuration structure for Searqon.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	SearXNG  SearXNGConfig  `yaml:"searxng"`
	Database DatabaseConfig `yaml:"database"`
	Redis    RedisConfig    `yaml:"redis"`
	Logs     LogsConfig     `yaml:"logs"`
	Cache    CacheConfig    `yaml:"cache"`
	Ollama   OllamaConfig   `yaml:"ollama"`
	Scraper  ScraperConfig  `yaml:"scraper"`
	Proxies  ProxiesConfig  `yaml:"proxies"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configMu     sync.RWMutex
)

// DefaultConfig returns safe and production-ready configuration defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:                4001,
			Host:                "0.0.0.0",
			ReadTimeoutSeconds:  30,
			WriteTimeoutSeconds: 120,
		},
		SearXNG: SearXNGConfig{
			Enabled: true,
			URL:     "http://searxng:8080",
			Port:    8080,
		},
		Database: DatabaseConfig{
			Enabled: true,
			Type:    "sqlite",
			SQLite: SQLiteConfig{
				Path: "./data/searqon.db",
			},
			Postgres: PostgresConfig{
				URL:      "postgres://searqon:searqon_pass@searqon-db:5432/searqon_cache?sslmode=disable",
				Host:     "searqon-db",
				Port:     5432,
				User:     "searqon",
				Password: "searqon_pass",
				DBName:   "searqon_cache",
				SSLMode:  "disable",
			},
		},
		Redis: RedisConfig{
			Enabled:   false,
			Host:      "searqon-redis",
			Port:      6379,
			Password:  "",
			DB:        0,
			KeyPrefix: "searqon:",
		},
		Logs: LogsConfig{
			Enabled:       true,
			Storage:       "sqlite",
			SQLitePath:    "./data/logs.db",
			Level:         "info",
			RetentionDays: 14,
		},
		Cache: CacheConfig{
			SearchTTLHours: 24,
			ScrapeTTLDays:  7,
		},
		Ollama: OllamaConfig{
			Enabled:        true,
			URL:            "http://host.docker.internal:11434",
			Model:          "gemma4",
			EmbeddingModel: "nomic-embed-text",
		},
		Scraper: ScraperConfig{
			RateLimitPerDomainRPS: 5,
			MaxConcurrentScrapes:  10,
			Lightpanda: BrowserEngineConfig{
				Enabled:    true,
				BinaryPath: "./lightpanda/lightpanda",
			},
			Camoufox: BrowserEngineConfig{
				Enabled:    false,
				BinaryPath: "/usr/local/bin/camoufox",
			},
		},
		Proxies: ProxiesConfig{
			Rotating:           []string{},
			ResidentialURL:     "",
			ResidentialProxies: []string{},
		},
	}
}

// LoadConfig loads configuration from YAML (settings.yml or config.yml) with env overrides.
func LoadConfig() *Config {
	configMu.Lock()
	defer configMu.Unlock()

	cfg := DefaultConfig()

	// Candidate paths to search for YAML settings
	candidateFiles := []string{
		os.Getenv("SEARQON_CONFIG"),
		"config/settings.yml",
		"config/settings.yaml",
		"../config/settings.yml",
		"../config/settings.yaml",
		"settings.yml",
		"settings.yaml",
		"config.yml",
		"config.yaml",
		"../settings.yml",
		"../config.yml",
	}

	var foundPath string
	for _, p := range candidateFiles {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			foundPath = p
			break
		}
	}

	if foundPath != "" {
		data, err := os.ReadFile(foundPath)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				log.Printf("[Config] Warning: Failed to parse %s: %v. Using defaults.", foundPath, err)
			} else {
				log.Printf("[Config] Successfully loaded settings from %s", foundPath)
			}
		}
	} else {
		log.Println("[Config] No settings.yml found, using default configuration.")
	}

	// Environment variable overrides for backward compatibility
	applyEnvOverrides(cfg)

	// Ensure SearXNG URL sync with port
	if cfg.SearXNG.Port > 0 && (cfg.SearXNG.URL == "" || cfg.SearXNG.URL == "http://localhost:4002") {
		cfg.SearXNG.URL = fmt.Sprintf("http://localhost:%d", cfg.SearXNG.Port)
	}

	// Ensure directories for SQLite databases exist
	if cfg.Database.Enabled && strings.EqualFold(cfg.Database.Type, "sqlite") && cfg.Database.SQLite.Path != "" {
		dir := filepath.Dir(cfg.Database.SQLite.Path)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
	}
	if cfg.Logs.Enabled && cfg.Logs.SQLitePath != "" {
		dir := filepath.Dir(cfg.Logs.SQLitePath)
		if dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}
	}

	globalConfig = cfg
	return cfg
}

// Get returns the loaded configuration, initializing if not already done.
func Get() *Config {
	configMu.RLock()
	cfg := globalConfig
	configMu.RUnlock()

	if cfg != nil {
		return cfg
	}

	configOnce.Do(func() {
		LoadConfig()
	})

	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// SearchCacheTTL returns duration for search cache expiration.
func (c *Config) SearchCacheTTL() time.Duration {
	if c.Cache.SearchTTLHours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(c.Cache.SearchTTLHours) * time.Hour
}

// ScrapeCacheTTL returns duration for scrape cache expiration.
func (c *Config) ScrapeCacheTTL() time.Duration {
	if c.Cache.ScrapeTTLDays <= 0 {
		return 7 * 24 * time.Hour
	}
	return time.Duration(c.Cache.ScrapeTTLDays) * 24 * time.Hour
}

func applyEnvOverrides(cfg *Config) {
	if p := os.Getenv("PORT"); p != "" {
		if portNum, err := strconv.Atoi(p); err == nil && portNum > 0 {
			cfg.Server.Port = portNum
		}
	}
	if u := os.Getenv("SEARXNG_URL"); u != "" {
		cfg.SearXNG.URL = u
		cfg.SearXNG.Enabled = true
	}
	if d := os.Getenv("DATABASE_URL"); d != "" {
		cfg.Database.Enabled = true
		cfg.Database.Type = "postgres"
		cfg.Database.Postgres.URL = d
	}
	if rHost := os.Getenv("REDIS_HOST"); rHost != "" {
		cfg.Redis.Enabled = true
		cfg.Redis.Host = rHost
	}
	if rPort := os.Getenv("REDIS_PORT"); rPort != "" {
		if p, err := strconv.Atoi(rPort); err == nil {
			cfg.Redis.Port = p
		}
	}
	if oURL := os.Getenv("OLLAMA_HOST"); oURL != "" {
		cfg.Ollama.URL = oURL
	}
	if oURL := os.Getenv("OLLAMA_URL"); oURL != "" {
		cfg.Ollama.URL = oURL
	}
	if oModel := os.Getenv("OLLAMA_MODEL"); oModel != "" {
		cfg.Ollama.Model = oModel
	}
	if embModel := os.Getenv("OLLAMA_EMBEDDING_MODEL"); embModel != "" {
		cfg.Ollama.EmbeddingModel = embModel
	}
	if rot := os.Getenv("ROTATING_PROXIES"); rot != "" {
		cfg.Proxies.Rotating = strings.Split(rot, ",")
	}
	if resURL := os.Getenv("RESIDENTIAL_PROXY_URL"); resURL != "" {
		cfg.Proxies.ResidentialURL = resURL
	}
	if resList := os.Getenv("RESIDENTIAL_PROXIES"); resList != "" {
		cfg.Proxies.ResidentialProxies = strings.Split(resList, ",")
	}
}
