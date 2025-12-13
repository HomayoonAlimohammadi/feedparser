package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// FeedConfig holds feed parsing and rate limiting settings.
type FeedConfig struct {
	TitleCharLimit       int           `json:"title_char_limit"`
	SummaryCharLimit     int           `json:"summary_char_limit"`
	SinceDaysBefore      int           `json:"since_days_before"`
	WaitPerEntry         time.Duration `json:"-"`
	WaitPerSource        time.Duration `json:"-"`
	WaitPerEntrySeconds  float64       `json:"wait_per_entry_seconds"`
	WaitPerSourceSeconds float64       `json:"wait_per_source_seconds"`
}

// AIConfig holds AI summarization settings.
type AIConfig struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"` // "openai" or "gemini"
	Model        string `json:"model"`
	APIKey       string `json:"api_key"`
	MaxTokens    int    `json:"max_tokens"`
	CacheEnabled bool   `json:"cache_enabled"`
	CacheDir     string `json:"cache_dir"`
	MaxRetries   int    `json:"max_retries"`
	RetryDelayMs int    `json:"retry_delay_ms"`
	RateLimitRPM int    `json:"rate_limit_rpm"` // requests per minute
}

// TelegramConfig holds Telegram bot settings.
type TelegramConfig struct {
	BotToken  string `json:"bot_access_token"`
	ChannelID string `json:"channel_id"`
}

// Source represents an RSS/Atom feed source.
type Source struct {
	Name    string
	FeedURL string
}

// Config is the root configuration structure.
type Config struct {
	Feed     FeedConfig     `json:"feed"`
	AI       AIConfig       `json:"ai"`
	Telegram TelegramConfig `json:"telegram"`
	Sources  []Source       `json:"-"`
}

// configJSON is used for JSON unmarshaling with sources as a map.
type configJSON struct {
	Feed     FeedConfig        `json:"feed"`
	AI       AIConfig          `json:"ai"`
	Telegram TelegramConfig    `json:"telegram"`
	Sources  map[string]string `json:"sources"`
}

// Load reads and parses the config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var raw configJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config JSON: %w", err)
	}

	cfg := &Config{
		Feed:     raw.Feed,
		AI:       raw.AI,
		Telegram: raw.Telegram,
		Sources:  make([]Source, 0, len(raw.Sources)),
	}

	// Apply defaults
	if cfg.Feed.TitleCharLimit == 0 {
		cfg.Feed.TitleCharLimit = 200
	}
	if cfg.Feed.SummaryCharLimit == 0 {
		cfg.Feed.SummaryCharLimit = 300
	}
	if cfg.Feed.SinceDaysBefore == 0 {
		cfg.Feed.SinceDaysBefore = 1
	}
	if cfg.Feed.WaitPerEntrySeconds == 0 {
		cfg.Feed.WaitPerEntrySeconds = 0.1
	}
	if cfg.Feed.WaitPerSourceSeconds == 0 {
		cfg.Feed.WaitPerSourceSeconds = 0.5
	}

	// AI defaults
	if cfg.AI.MaxTokens == 0 {
		cfg.AI.MaxTokens = 1024
	}
	if cfg.AI.CacheDir == "" {
		cfg.AI.CacheDir = ".cache/summaries"
	}
	if cfg.AI.MaxRetries == 0 {
		cfg.AI.MaxRetries = 3
	}
	if cfg.AI.RetryDelayMs == 0 {
		cfg.AI.RetryDelayMs = 1000
	}
	if cfg.AI.RateLimitRPM == 0 {
		cfg.AI.RateLimitRPM = 60
	}

	// Convert seconds to Duration
	cfg.Feed.WaitPerEntry = time.Duration(cfg.Feed.WaitPerEntrySeconds * float64(time.Second))
	cfg.Feed.WaitPerSource = time.Duration(cfg.Feed.WaitPerSourceSeconds * float64(time.Second))

	// Convert sources map to slice
	for name, url := range raw.Sources {
		cfg.Sources = append(cfg.Sources, Source{
			Name:    name,
			FeedURL: url,
		})
	}

	return cfg, nil
}
