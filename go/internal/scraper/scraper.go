package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Scraper fetches and extracts article content from URLs.
type Scraper interface {
	// Scrape fetches the URL and extracts the main article content.
	Scrape(ctx context.Context, url string) (string, error)
}

// HTMLScraper implements Scraper by fetching HTML and extracting article content.
type HTMLScraper struct {
	client *http.Client
	logger *slog.Logger
}

// NewScraper creates a new HTML scraper.
func NewScraper(logger *slog.Logger) Scraper {
	return &HTMLScraper{
		client: &http.Client{Timeout: 30 * time.Second},
		logger: logger,
	}
}

// Scrape fetches the URL and extracts the main article content.
func (s *HTMLScraper) Scrape(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedParser/1.0; +https://github.com/HomayoonAlimohammadi/feedparser)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	content, err := s.extractContent(string(body))
	if err != nil {
		return "", fmt.Errorf("extracting content: %w", err)
	}

	s.logger.Debug("scraped article",
		slog.String("url", url),
		slog.Int("content_length", len(content)),
	)

	return content, nil
}

// extractContent extracts the main article content from HTML.
func (s *HTMLScraper) extractContent(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", fmt.Errorf("parsing HTML: %w", err)
	}

	// Remove unwanted elements
	doc.Find("script, style, nav, header, footer, aside, .sidebar, .comments, .advertisement, .ad, #comments, .social-share, .related-posts").Remove()

	// Try to find the main article content using common selectors
	var content string

	// Priority order of selectors for article content
	selectors := []string{
		"article",
		"[role='main']",
		".post-content",
		".article-content",
		".entry-content",
		".content",
		"main",
		".post",
		".blog-post",
		"#content",
	}

	for _, selector := range selectors {
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			content = s.extractText(selection.First())
			if len(content) > 200 { // Minimum content length
				break
			}
		}
	}

	// Fallback to body if no article found
	if content == "" {
		content = s.extractText(doc.Find("body"))
	}

	// Clean up the content
	content = s.cleanText(content)

	// Limit content length to avoid token limits
	const maxChars = 15000
	if len(content) > maxChars {
		content = content[:maxChars] + "..."
	}

	return content, nil
}

// extractText extracts text content from a selection, preserving some structure.
func (s *HTMLScraper) extractText(selection *goquery.Selection) string {
	var sb strings.Builder

	selection.Find("h1, h2, h3, h4, h5, h6, p, li, pre, code, blockquote").Each(func(i int, sel *goquery.Selection) {
		tag := goquery.NodeName(sel)
		text := strings.TrimSpace(sel.Text())

		if text == "" {
			return
		}

		switch tag {
		case "h1", "h2", "h3", "h4", "h5", "h6":
			sb.WriteString("\n## ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		case "pre", "code":
			// Check if it's a code block
			if sel.Parent().Is("pre") || tag == "pre" {
				lang := sel.AttrOr("class", "")
				sb.WriteString("\n```")
				sb.WriteString(extractLanguage(lang))
				sb.WriteString("\n")
				sb.WriteString(text)
				sb.WriteString("\n```\n\n")
			} else {
				sb.WriteString("`")
				sb.WriteString(text)
				sb.WriteString("`")
			}
		case "li":
			sb.WriteString("- ")
			sb.WriteString(text)
			sb.WriteString("\n")
		case "blockquote":
			sb.WriteString("> ")
			sb.WriteString(text)
			sb.WriteString("\n\n")
		default:
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	})

	return sb.String()
}

// extractLanguage tries to extract programming language from class attribute.
func extractLanguage(class string) string {
	// Common patterns: language-go, lang-python, highlight-javascript
	patterns := []string{
		`language-(\w+)`,
		`lang-(\w+)`,
		`highlight-(\w+)`,
		`(\w+)-highlight`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(class)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// cleanText cleans up extracted text.
func (s *HTMLScraper) cleanText(text string) string {
	// Replace multiple newlines with double newline
	re := regexp.MustCompile(`\n{3,}`)
	text = re.ReplaceAllString(text, "\n\n")

	// Replace multiple spaces with single space
	re = regexp.MustCompile(`[ \t]+`)
	text = re.ReplaceAllString(text, " ")

	// Trim whitespace from each line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	text = strings.Join(lines, "\n")

	return strings.TrimSpace(text)
}
