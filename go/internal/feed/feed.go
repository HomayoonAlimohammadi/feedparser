package feed

import (
	"fmt"
	"strings"

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
