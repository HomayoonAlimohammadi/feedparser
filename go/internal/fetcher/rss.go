package fetcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HomayoonAlimohammadi/feedparser/internal/scraper"
	"github.com/mmcdole/gofeed"
)

// RSSFetcher fetches items from RSS/Atom feeds.
type RSSFetcher struct {
	name   string
	url    string
	logger *slog.Logger
	scr    scraper.Scraper
}

// NewRSSFetcher creates a new RSS fetcher.
func NewRSSFetcher(name, url string, logger *slog.Logger, scr scraper.Scraper) *RSSFetcher {
	return &RSSFetcher{
		name:   name,
		url:    url,
		logger: logger,
		scr:    scr,
	}
}

// Name returns the source name.
func (f *RSSFetcher) Name() string {
	return f.name
}

// Fetch retrieves and parses the RSS feed.
func (f *RSSFetcher) Fetch(ctx context.Context, since time.Time) ([]Item, error) {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURLWithContext(f.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("parsing feed from URL: %w", err)
	}

	f.logger.Debug("fetched RSS feed",
		slog.String("source", f.name),
		slog.Int("items", len(parsed.Items)),
	)

	items := make([]Item, 0)
	for _, entry := range parsed.Items {
		var publishDate time.Time
		if entry.PublishedParsed != nil {
			publishDate = *entry.PublishedParsed
		} else if entry.UpdatedParsed != nil {
			publishDate = *entry.UpdatedParsed
		} else {
			continue // Skip entries without dates
		}

		if publishDate.Before(since) {
			continue
		}

		content, err := f.scr.Scrape(ctx, entry.Link)
		if err != nil {
			f.logger.Warn("failed to scrape article, using description",
				slog.String("title", entry.Title),
				slog.String("error", err.Error()),
			)
			content = entry.Description
		}

		items = append(items, Item{
			Title:          entry.Title,
			Description:    entry.Description,
			Link:           entry.Link,
			PublishDate:    publishDate,
			Source:         f.name,
			ScrapedContent: content,
		})
	}

	return items, nil
}
