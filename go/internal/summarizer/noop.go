package summarizer

import (
	"context"

	"github.com/HomayoonAlimohammadi/feedparser/internal/fetcher"
)

// NoopSummarizer is a no-op implementation that returns empty summaries.
// Used when AI summarization is disabled.
type NoopSummarizer struct{}

// NewNoopSummarizer creates a new no-op summarizer.
func NewNoopSummarizer() *NoopSummarizer {
	return &NoopSummarizer{}
}

// Summarize returns an empty summary (no-op).
func (s *NoopSummarizer) Summarize(ctx context.Context, item fetcher.Item) (*Summary, error) {
	return &Summary{}, nil
}

// SummarizeBatch returns an empty batch summary (no-op).
func (s *NoopSummarizer) SummarizeBatch(ctx context.Context, sourceName string, items []fetcher.Item) (*BatchSummary, error) {
	return &BatchSummary{}, nil
}
