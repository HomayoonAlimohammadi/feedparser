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

// Reddit JSON API response structures
type redditResponse struct {
	Kind string         `json:"kind"`
	Data redditListData `json:"data"`
}

type redditListData struct {
	After    string       `json:"after"`
	Before   string       `json:"before"`
	Children []redditPost `json:"children"`
}

type redditPost struct {
	Kind string         `json:"kind"`
	Data redditPostData `json:"data"`
}

type redditPostData struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Selftext      string  `json:"selftext"`
	URL           string  `json:"url"`
	Permalink     string  `json:"permalink"`
	Author        string  `json:"author"`
	Score         int     `json:"score"`
	NumComments   int     `json:"num_comments"`
	CreatedUTC    float64 `json:"created_utc"`
	Subreddit     string  `json:"subreddit"`
	IsSelf        bool    `json:"is_self"`
	LinkFlairText string  `json:"link_flair_text"`
	Over18        bool    `json:"over_18"`
	Stickied      bool    `json:"stickied"`
}

// RedditFetcher fetches items from Reddit subreddits or searches.
type RedditFetcher struct {
	subreddits []string
	keywords   []string
	client     *http.Client
	logger     *slog.Logger
	minScore   int
	useSearch  bool // Use search API instead of subreddit feeds
	scr        scraper.Scraper
}

// RedditOption configures the RedditFetcher.
type RedditOption func(*RedditFetcher)

// WithSubreddits sets specific subreddits to monitor.
func WithSubreddits(subs []string) RedditOption {
	return func(f *RedditFetcher) {
		f.subreddits = subs
	}
}

// WithRedditKeywords sets keywords to search for.
func WithRedditKeywords(keywords []string) RedditOption {
	return func(f *RedditFetcher) {
		f.keywords = keywords
		f.useSearch = true
	}
}

// WithMinScore sets the minimum score threshold.
func WithMinScore(score int) RedditOption {
	return func(f *RedditFetcher) {
		f.minScore = score
	}
}

// NewRedditFetcher creates a new Reddit fetcher.
// Can be configured to either watch specific subreddits or search for keywords.
func NewRedditFetcher(client *http.Client, scr scraper.Scraper, logger *slog.Logger, opts ...RedditOption) *RedditFetcher {
	f := &RedditFetcher{
		client:   client,
		logger:   logger,
		minScore: 5, // Default minimum score
		scr:      scr,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// Name returns the source name.
func (f *RedditFetcher) Name() string {
	// if f.useSearch && len(f.keywords) > 0 {
	// 	return fmt.Sprintf("Reddit Search (%s)", strings.Join(f.keywords, ", "))
	// }
	// if len(f.subreddits) > 0 {
	// 	return fmt.Sprintf("Reddit (r/%s)", strings.Join(f.subreddits, ", r/"))
	// }
	return "Reddit"
}

// Fetch retrieves posts from Reddit.
func (f *RedditFetcher) Fetch(ctx context.Context, since time.Time) ([]Item, error) {
	items := make([]Item, 0)
	seen := make(map[string]bool)

	// Fetch from subreddits
	for _, sub := range f.subreddits {
		subItems, err := f.fetchSubreddit(ctx, sub, since)
		if err != nil {
			f.logger.Warn("failed to fetch subreddit",
				slog.String("subreddit", sub),
				slog.String("error", err.Error()),
			)
			continue
		}

		for _, item := range subItems {
			if seen[item.Link] {
				continue
			}
			seen[item.Link] = true
			items = append(items, item)
		}
	}

	// Search for keywords
	if f.useSearch {
		for _, keyword := range f.keywords {
			searchItems, err := f.searchKeyword(ctx, keyword, since)
			if err != nil {
				f.logger.Warn("failed to search Reddit",
					slog.String("keyword", keyword),
					slog.String("error", err.Error()),
				)
				continue
			}

			for _, item := range searchItems {
				if seen[item.Link] {
					continue
				}
				seen[item.Link] = true
				item.Keywords = append(item.Keywords, keyword)
				items = append(items, item)
			}
		}
	}

	// f.logger.Info("fetched from Reddit",
	// 	slog.Int("items", len(items)),
	// )

	return items, nil
}

func (f *RedditFetcher) fetchSubreddit(ctx context.Context, subreddit string, since time.Time) ([]Item, error) {
	// Use Reddit JSON API (append .json to any Reddit URL)
	reqURL := fmt.Sprintf("https://www.reddit.com/r/%s/new.json?limit=100", subreddit)

	return f.fetchRedditURL(ctx, reqURL, since, subreddit)
}

func (f *RedditFetcher) searchKeyword(ctx context.Context, keyword string, since time.Time) ([]Item, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("sort", "new")
	params.Set("limit", "100")
	params.Set("t", "week") // Time filter: hour, day, week, month, year, all

	reqURL := fmt.Sprintf("https://www.reddit.com/search.json?%s", params.Encode())

	return f.fetchRedditURL(ctx, reqURL, since, "search")
}

func (f *RedditFetcher) fetchRedditURL(ctx context.Context, reqURL string, since time.Time, source string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Reddit requires a User-Agent
	req.Header.Set("User-Agent", "feedparser/1.0 (https://github.com/HomayoonAlimohammadi/feedparser)")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Reddit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var redditResp redditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	f.logger.Debug("Reddit API response",
		slog.String("source", source),
		slog.Int("posts", len(redditResp.Data.Children)),
	)

	items := make([]Item, 0)
	for _, post := range redditResp.Data.Children {
		data := post.Data

		// Skip NSFW, stickied, or low-score posts
		if data.Over18 || data.Stickied || data.Score < f.minScore {
			continue
		}

		publishDate := time.Unix(int64(data.CreatedUTC), 0)
		if publishDate.Before(since) {
			continue
		}

		// Use external link if available, otherwise Reddit permalink
		link := data.URL
		if data.IsSelf {
			link = "https://www.reddit.com" + data.Permalink
		}

		description := data.Selftext
		if description == "" {
			description = fmt.Sprintf("r/%s | Score: %d | Comments: %d", data.Subreddit, data.Score, data.NumComments)
		} else if len(description) > 500 {
			description = description[:500] + "..."
		}

		content, err := f.scr.Scrape(ctx, link)
		if err != nil {
			f.logger.Warn("failed to scrape article, using description",
				slog.String("title", data.Title),
				slog.String("error", err.Error()),
			)
			content = description
		}

		items = append(items, Item{
			Title:          data.Title,
			Description:    description,
			Link:           link,
			PublishDate:    publishDate,
			Source:         fmt.Sprintf("Reddit (r/%s)", data.Subreddit),
			Score:          data.Score,
			Comments:       data.NumComments,
			ScrapedContent: content,
		})
	}

	return items, nil
}

// GetRedditDiscussionURL returns the Reddit discussion URL for a post.
func GetRedditDiscussionURL(permalink string) string {
	return "https://www.reddit.com" + permalink
}
