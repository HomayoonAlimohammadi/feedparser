package summarizer

import (
	"context"
)

// Summary represents an AI-generated summary of an article.
type Summary struct {
	// TLDR is a single paragraph summary
	TLDR string `json:"tldr"`
	// FullSummary is a detailed multi-paragraph summary in Markdown
	FullSummary string `json:"full_summary"`
	// ExtractedLinks are relevant links found in the article
	ExtractedLinks []Link `json:"extracted_links"`
	// CodeSnippets are important code examples from the article
	CodeSnippets []CodeSnippet `json:"code_snippets,omitempty"`
}

// Link represents an extracted link from the article.
type Link struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// CodeSnippet represents a code example from the article.
type CodeSnippet struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Context  string `json:"context"` // Brief description of what the code does
}

// Summarizer is the interface for AI-powered article summarization.
type Summarizer interface {
	// Summarize generates a summary from article content.
	// content is the full article text (from scraper).
	// title is the article title for context.
	Summarize(ctx context.Context, title, content, articleURL string) (*Summary, error)
}

// DefaultPrompt is the system prompt for summarization.
const DefaultPrompt = `You are a technical content summarizer. Your task is to create high-quality summaries of technical blog posts and articles.

For each article, provide:

1. **TLDR**: A single paragraph (2-4 sentences) capturing the key insight or main point.

2. **Full Summary**: A detailed summary in Markdown format that:
   - Preserves important technical details and concepts
   - Uses multiple paragraphs as needed
   - Includes bullet points for lists of features/steps
   - Maintains technical accuracy
   - Highlights key takeaways

3. **Extracted Links**: Important links mentioned in the article (documentation, tools, related resources).

4. **Code Snippets**: If the article contains important code examples, include the most relevant ones with brief context.

Guidelines:
- Be concise but thorough
- Preserve technical terminology
- Include specific numbers, benchmarks, or metrics if mentioned
- Focus on actionable insights
- Use Markdown formatting for readability

Respond in JSON format exactly like below (no leading or trailing text):
{
  "tldr": "...",
  "full_summary": "...",
  "extracted_links": [{"text": "...", "url": "..."}],
  "code_snippets": [{"language": "...", "code": "...", "context": "..."}]
}`
