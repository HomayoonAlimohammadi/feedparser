package summarizer

import (
	"context"

	"github.com/HomayoonAlimohammadi/feedparser/internal/fetcher"
)

// Summary represents an AI-generated summary of an article.
type Summary struct {
	// TLDR is a single paragraph summary
	TLDR string `json:"tldr"`
	// // FullSummary is a detailed multi-paragraph summary in Markdown
	// FullSummary string `json:"full_summary"`
	// // ExtractedLinks are relevant links found in the article
	// ExtractedLinks []Link `json:"extracted_links"`
	// // CodeSnippets are important code examples from the article
	// CodeSnippets []CodeSnippet `json:"code_snippets,omitempty"`
}

// BatchItem represents a single item in a batch summary request.
type BatchItem struct {
	Title       string
	Description string
	Link        string
}

// BatchSummary represents an AI-generated summary of multiple items from a source.
type BatchSummary struct {
	// Overview is a high-level summary of all items from the source
	Overview string `json:"overview"`
}

// // Link represents an extracted link from the article.
// type Link struct {
// 	Text string `json:"text"`
// 	URL  string `json:"url"`
// }

// // CodeSnippet represents a code example from the article.
// type CodeSnippet struct {
// 	Language string `json:"language"`
// 	Code     string `json:"code"`
// 	Context  string `json:"context"` // Brief description of what the code does
// }

// Summarizer is the interface for AI-powered article summarization.
type Summarizer interface {
	// Summarize generates a summary from article content.
	// content is the full article text (from scraper).
	// title is the article title for context.
	Summarize(ctx context.Context, item fetcher.Item) (*Summary, error)

	// SummarizeBatch generates a digest summary of multiple items from a source.
	// sourceName is the name of the fetcher (e.g., "Hacker News", "Reddit").
	// items are the items to summarize together.
	SummarizeBatch(ctx context.Context, sourceName string, items []fetcher.Item) (*BatchSummary, error)
}

// DefaultPrompt is the system prompt for summarization.
const DefaultPrompt = `You are a technical content summarizer. Your task is to create high-quality summaries of technical blog posts and articles.

For each article, provide a **TLDR**, a single paragraph (around 2-3 sentences) capturing the key insight or main point.

Guidelines:
- Be concise but thorough
- Preserve technical terminology
- Include specific numbers, benchmarks, or metrics if mentioned
- Focus on actionable insights
- Use plain text formatting
- VERY IMPORTANT: Output only the summary without any leading or trailing text. To the point.
`

// BatchPrompt is the system prompt for batch/digest summarization.
const BatchPrompt = `You are a technical content curator. Your task is to create a digest summary of multiple items from a single source.

Given a list of article titles and descriptions, provide an **Overview** which is a concise paragraph (8-15 sentences) summarizing the main topics and trends across all items.

Guidelines:
- Identify common patterns and trending topics
- Highlight the most significant or interesting items
- Be concise and informative
- Use plain text formatting
- VERY IMPOTANT: As your response, ONLY output the summary, no leading or trailing text or description. 
`
