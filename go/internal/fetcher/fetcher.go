package fetcher

import (
	"context"
	"time"
)

// Item represents a generic content item from any source.
type Item struct {
	Title          string
	Description    string
	Link           string
	PublishDate    time.Time
	Source         string
	Score          int      // For HN/Reddit upvotes
	Comments       int      // Comment count
	Keywords       []string // Matched keywords
	ScrapedContent string   // Content scraped from the link
}

// Fetcher is the interface for fetching content from different sources.
type Fetcher interface {
	// Fetch retrieves items from the source, filtered by since time.
	Fetch(ctx context.Context, since time.Time) ([]Item, error)
	// Name returns the source name for display.
	Name() string
}
