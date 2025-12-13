package feed

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/HomayoonAlimohammadi/feedparser/internal/config"
	"github.com/HomayoonAlimohammadi/feedparser/internal/summarizer"
)

// Feed represents a parsed feed entry ready for broadcasting.
type Feed struct {
	Source      string
	Title       string
	Summary     string // Legacy: truncated RSS summary
	Link        string
	PublishDate string
	Keywords    []string

	// AI-generated summary (populated if AI is enabled)
	AISummary *summarizer.Summary
}

// String returns a human-readable representation of the feed.
func (f Feed) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Source: %s\n", f.Source))
	sb.WriteString(fmt.Sprintf("Title: %s\n", f.Title))
	if f.AISummary != nil && f.AISummary.TLDR != "" {
		sb.WriteString(fmt.Sprintf("TLDR: %s\n", f.AISummary.TLDR))
	} else if f.Summary != "" {
		sb.WriteString(fmt.Sprintf("Summary: %s\n", f.Summary))
	}
	sb.WriteString(fmt.Sprintf("Published At: %s\n", f.PublishDate))
	sb.WriteString(fmt.Sprintf("\n%s", f.Link))
	return sb.String()
}

// Parser is the interface for parsing feed content.
type Parser interface {
	Parse(ctx context.Context, sourceName, content string, since time.Time) ([]Feed, error)
}

// GofeedParser handles RSS/Atom feed parsing using gofeed library.
type GofeedParser struct {
	cfg    config.FeedConfig
	logger *slog.Logger
}

// NewParser creates a new feed parser with the given config.
func NewParser(cfg config.FeedConfig, logger *slog.Logger) Parser {
	return &GofeedParser{
		cfg:    cfg,
		logger: logger,
	}
}

// Parse fetches and parses feeds from the given content.
func (p *GofeedParser) Parse(ctx context.Context, sourceName, content string, since time.Time) ([]Feed, error) {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseString(content)
	if err != nil {
		return nil, fmt.Errorf("parsing feed content: %w", err)
	}

	p.logger.Info("parsing source",
		slog.String("source", sourceName),
		slog.Int("entries", len(parsed.Items)),
	)

	feeds := make([]Feed, 0)

	// Process in reverse order (oldest first, like Python version)
	for i := len(parsed.Items) - 1; i >= 0; i-- {
		item := parsed.Items[i]

		// Parse publish date
		var publishDate time.Time
		if item.PublishedParsed != nil {
			publishDate = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			publishDate = *item.UpdatedParsed
		} else {
			p.logger.Warn("skipping entry with no date",
				slog.String("title", item.Title),
			)
			continue
		}

		title := item.Title
		if len(title) > p.cfg.TitleCharLimit {
			title = title[:p.cfg.TitleCharLimit] + "..."
		}

		// Skip old entries (with special case for "The Go Blog")
		if !p.isRecent(sourceName, publishDate, since, title) {
			continue
		}

		summary := item.Description
		if len(summary) > p.cfg.SummaryCharLimit {
			summary = summary[:p.cfg.SummaryCharLimit] + "..."
		}

		// Language detection (placeholder - always returns true for now)
		if !isEnglish(title) {
			p.logger.Debug("skipping non-English entry",
				slog.String("title", title),
			)
			continue
		}

		feeds = append(feeds, Feed{
			Source:      sourceName,
			Title:       title,
			Summary:     summary,
			Link:        item.Link,
			PublishDate: FormatTime(publishDate),
		})
	}

	p.logger.Info("parsed feeds",
		slog.String("source", sourceName),
		slog.Int("count", len(feeds)),
	)

	return feeds, nil
}

// isRecent checks if the entry is recent enough to include.
// Special case: "The Go Blog" has a 2-day grace period.
func (p *GofeedParser) isRecent(sourceName string, publishDate, since time.Time, title string) bool {
	if sourceName == "The Go Blog" {
		gracePeriod := since.Add(-2 * 24 * time.Hour)
		if publishDate.Before(gracePeriod) {
			p.logger.Info("entry too old",
				slog.String("title", title),
				slog.String("published", FormatTime(publishDate)),
			)
			return false
		}
		p.logger.Info("The Go Blog tricky publish",
			slog.String("title", title),
			slog.String("published", FormatTime(publishDate)),
		)
		return true
	}

	if publishDate.Before(since) {
		p.logger.Info("entry too old",
			slog.String("title", title),
			slog.String("published", FormatTime(publishDate)),
		)
		return false
	}
	return true
}

// isEnglish is a placeholder for language detection.
// TODO: Implement actual language detection if needed.
func isEnglish(text string) bool {
	// Placeholder: always returns true
	// To implement: use github.com/pemistahl/lingua-go or similar
	return true
}

// FormatTime formats a time.Time to a human-readable string.
// Matches the Python display_time format: "December 8th 2025 14:30:0"
func FormatTime(t time.Time) string {
	months := []string{
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	day := t.Day()
	suffix := "th"
	if day < 11 || day > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}

	return fmt.Sprintf("%s %d%s %d %d:%02d:%d",
		months[t.Month()],
		day, suffix,
		t.Year(),
		t.Hour(), t.Minute(), t.Second(),
	)
}
