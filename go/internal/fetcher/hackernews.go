package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/HomayoonAlimohammadi/feedparser/internal/scraper"
)

// HackerNews Algolia API response structures
type hnSearchResponse struct {
	Hits             []hnHit `json:"hits"`
	NbHits           int     `json:"nbHits"`
	Page             int     `json:"page"`
	NbPages          int     `json:"nbPages"`
	HitsPerPage      int     `json:"hitsPerPage"`
	ProcessingTimeMS int     `json:"processingTimeMS"`
}

type hnHit struct {
	ObjectID    string   `json:"objectID"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Author      string   `json:"author"`
	Points      int      `json:"points"`
	NumComments int      `json:"num_comments"`
	CreatedAt   string   `json:"created_at"`   // ISO 8601 format
	CreatedAtI  int64    `json:"created_at_i"` // Unix timestamp
	StoryText   string   `json:"story_text"`   // For Ask HN posts
	Tags        []string `json:"_tags"`
}

// HackerNewsFetcher fetches items from Hacker News using the Algolia API.
type HackerNewsFetcher struct {
	keywords      []string
	client        *http.Client
	logger        *slog.Logger
	minPoints     int  // Minimum upvotes to include
	frontPageOnly bool // Only fetch front page stories
	scr           scraper.Scraper
}

// HNOption configures the HackerNewsFetcher.
type HNOption func(*HackerNewsFetcher)

// WithMinPoints sets the minimum points threshold.
func WithMinPoints(points int) HNOption {
	return func(f *HackerNewsFetcher) {
		f.minPoints = points
	}
}

// WithFrontPageOnly limits to front page stories only.
func WithFrontPageOnly(frontPage bool) HNOption {
	return func(f *HackerNewsFetcher) {
		f.frontPageOnly = frontPage
	}
}

// NewHackerNewsFetcher creates a new Hacker News fetcher for specific keywords.
func NewHackerNewsFetcher(keywords []string, client *http.Client, scr scraper.Scraper, logger *slog.Logger, opts ...HNOption) *HackerNewsFetcher {
	f := &HackerNewsFetcher{
		keywords:  keywords,
		client:    client,
		logger:    logger,
		minPoints: 10, // Default minimum points
		scr:       scr,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// Name returns the source name.
func (f *HackerNewsFetcher) Name() string {
	// return fmt.Sprintf("Hacker News (%s)", strings.Join(f.keywords, ", "))
	return "Hacker News"
}

// Fetch retrieves stories from Hacker News matching the keywords.
func (f *HackerNewsFetcher) Fetch(ctx context.Context, since time.Time) ([]Item, error) {
	items := make([]Item, 0)
	seen := make(map[string]bool) // Deduplicate by URL

	for _, keyword := range f.keywords {
		keywordItems, err := f.fetchKeyword(ctx, keyword, since)
		if err != nil {
			f.logger.Warn("failed to fetch keyword from HN",
				slog.String("keyword", keyword),
				slog.String("error", err.Error()),
			)
			continue
		}

		for _, item := range keywordItems {
			if seen[item.Link] {
				continue
			}
			seen[item.Link] = true
			item.Keywords = append(item.Keywords, keyword)
			items = append(items, item)
		}
	}

	// f.logger.Info("fetched from Hacker News",
	// 	slog.Int("items", len(items)),
	// 	slog.Any("keywords", f.keywords),
	// )

	return items, nil
}

func (f *HackerNewsFetcher) fetchKeyword(ctx context.Context, keyword string, since time.Time) ([]Item, error) {
	// Use Algolia's HN Search API
	// Docs: https://hn.algolia.com/api
	baseURL := "https://hn.algolia.com/api/v1/search"

	params := url.Values{}
	params.Set("query", keyword)
	params.Set("tags", "story") // Only stories, not comments
	params.Set("numericFilters", fmt.Sprintf("created_at_i>%d,points>=%d", since.Unix(), f.minPoints))
	params.Set("hitsPerPage", "50")

	reqURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching HN: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp hnSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	f.logger.Debug("HN API response",
		slog.String("keyword", keyword),
		slog.Int("hits", searchResp.NbHits),
	)

	items := make([]Item, 0, len(searchResp.Hits))
	for _, hit := range searchResp.Hits {
		// Skip if no URL (self posts without content)
		link := hit.URL
		if link == "" {
			// For Ask HN, Show HN posts without external link, use the HN discussion
			link = fmt.Sprintf("https://news.ycombinator.com/item?id=%s", hit.ObjectID)
		}

		publishDate := time.Unix(hit.CreatedAtI, 0)

		description := hit.StoryText
		if description == "" {
			description = fmt.Sprintf("Points: %d | Comments: %d", hit.Points, hit.NumComments)
		}

		content, err := f.scr.Scrape(ctx, link)
		if err != nil {
			f.logger.Warn("failed to scrape article, using description",
				slog.String("title", hit.Title),
				slog.String("error", err.Error()),
			)
			content = description
		}

		items = append(items, Item{
			Title:          hit.Title,
			Description:    description,
			Link:           link,
			PublishDate:    publishDate,
			Source:         "Hacker News",
			Score:          hit.Points,
			Comments:       hit.NumComments,
			ScrapedContent: content,
		})
	}

	return items, nil
}

// GetHNDiscussionURL returns the HN discussion URL for an item.
func GetHNDiscussionURL(objectID string) string {
	return fmt.Sprintf("https://news.ycombinator.com/item?id=%s", objectID)
}
