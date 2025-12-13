package summarizer

import (
	"context"
)

// NoopSummarizer is a no-op implementation that returns empty summaries.
// Used when AI summarization is disabled.
type NoopSummarizer struct{}

// NewNoopSummarizer creates a new no-op summarizer.
func NewNoopSummarizer() *NoopSummarizer {
	return &NoopSummarizer{}
}

// Summarize returns an empty summary (no-op).
func (s *NoopSummarizer) Summarize(ctx context.Context, title, content, articleURL string) (*Summary, error) {
	return &Summary{}, nil
}
