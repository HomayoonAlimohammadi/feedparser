package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/HomayoonAlimohammadi/feedparser/internal/summarizer"
)

// Cache provides caching for article summaries.
type Cache interface {
	// Get retrieves a cached summary by article URL.
	Get(articleURL string) (*summarizer.Summary, bool)
	// Set stores a summary in the cache.
	Set(articleURL string, summary *summarizer.Summary) error
}

// FileCache implements Cache using the filesystem.
type FileCache struct {
	cacheDir string
	logger   *slog.Logger
}

// NewFileCache creates a new file-based cache.
func NewFileCache(cacheDir string, logger *slog.Logger) (*FileCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("creating cache directory: %w", err)
	}
	return &FileCache{
		cacheDir: cacheDir,
		logger:   logger,
	}, nil
}

// Get retrieves a cached summary by article URL.
func (c *FileCache) Get(articleURL string) (*summarizer.Summary, bool) {
	path := c.pathForURL(articleURL)

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			c.logger.Warn("failed to read cache file",
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
		}
		return nil, false
	}

	var summary summarizer.Summary
	if err := json.Unmarshal(data, &summary); err != nil {
		c.logger.Warn("failed to unmarshal cached summary",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return nil, false
	}

	c.logger.Debug("cache hit",
		slog.String("url", articleURL),
	)

	return &summary, true
}

// Set stores a summary in the cache.
func (c *FileCache) Set(articleURL string, summary *summarizer.Summary) error {
	path := c.pathForURL(articleURL)

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling summary: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing cache file: %w", err)
	}

	c.logger.Debug("cached summary",
		slog.String("url", articleURL),
		slog.String("path", path),
	)

	return nil
}

// pathForURL generates a cache file path for the given URL.
func (c *FileCache) pathForURL(url string) string {
	hash := sha256.Sum256([]byte(url))
	filename := hex.EncodeToString(hash[:]) + ".json"
	return filepath.Join(c.cacheDir, filename)
}

// NoopCache is a no-op cache implementation.
type NoopCache struct{}

// NewNoopCache creates a new no-op cache.
func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

// Get always returns false (no cache).
func (c *NoopCache) Get(articleURL string) (*summarizer.Summary, bool) {
	return nil, false
}

// Set does nothing.
func (c *NoopCache) Set(articleURL string, summary *summarizer.Summary) error {
	return nil
}
