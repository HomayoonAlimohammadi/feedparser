package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/HomayoonAlimohammadi/feedparser/internal/config"
	"github.com/HomayoonAlimohammadi/feedparser/internal/fetcher"
)

// OpenAISummarizer implements Summarizer using OpenAI's API.
type OpenAISummarizer struct {
	apiKey      string
	model       string
	maxTokens   int
	maxRetries  int
	retryDelay  time.Duration
	rateLimiter *rateLimiter
	client      *http.Client
	logger      *slog.Logger
}

// NewOpenAISummarizer creates a new OpenAI summarizer.
func NewOpenAISummarizer(cfg config.AIConfig, logger *slog.Logger) *OpenAISummarizer {
	return &OpenAISummarizer{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		maxRetries:  cfg.MaxRetries,
		retryDelay:  time.Duration(cfg.RetryDelayMs) * time.Millisecond,
		rateLimiter: newRateLimiter(cfg.RateLimitRPM),
		client:      &http.Client{Timeout: 120 * time.Second},
		logger:      logger,
	}
}

// openAIRequest represents the OpenAI API request body.
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse represents the OpenAI API response.
type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Summarize implements the Summarizer interface.
func (s *OpenAISummarizer) Summarize(ctx context.Context, item fetcher.Item) (*Summary, error) {
	// Wait for rate limiter
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	userPrompt := fmt.Sprintf("Article Title: %s\nArticle URL: %s\n\nArticle Content:\n%s", item.Title, item.Link, item.ScrapedContent)

	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Info("retrying OpenAI request",
				slog.Int("attempt", attempt+1),
				slog.String("title", item.Title),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.retryDelay * time.Duration(attempt+1)):
			}
		}

		summary, err := s.doRequest(ctx, userPrompt)
		if err == nil {
			return summary, nil
		}
		lastErr = err
		s.logger.Warn("OpenAI request failed",
			slog.String("error", err.Error()),
			slog.Int("attempt", attempt+1),
		)
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}

func (s *OpenAISummarizer) doRequest(ctx context.Context, userPrompt string) (*Summary, error) {
	reqBody := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{Role: "system", Content: DefaultPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   s.maxTokens,
		Temperature: 0.3,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	// Parse the JSON response from the model
	var summary Summary
	content := openAIResp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &summary); err != nil {
		// If JSON parsing fails, try to extract from markdown code block
		summary = Summary{
			TLDR: content,
			// FullSummary: content,
		}
		s.logger.Warn("failed to parse JSON response, using raw content",
			slog.String("error", err.Error()),
		)
	}

	return &summary, nil
}

// rateLimiter implements a simple token bucket rate limiter.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		tokens:     requestsPerMinute,
		maxTokens:  requestsPerMinute,
		refillRate: time.Minute / time.Duration(requestsPerMinute),
		lastRefill: time.Now(),
	}
}

func (r *rateLimiter) Wait(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed / r.refillRate)
	if tokensToAdd > 0 {
		r.tokens = min(r.tokens+tokensToAdd, r.maxTokens)
		r.lastRefill = now
	}

	// If no tokens available, wait
	if r.tokens <= 0 {
		waitTime := r.refillRate - now.Sub(r.lastRefill)
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
		r.mu.Lock()
		r.tokens = 1
	}

	r.tokens--
	return nil
}

// SummarizeBatch generates a digest summary of multiple items from a source.
func (s *OpenAISummarizer) SummarizeBatch(ctx context.Context, sourceName string, items []fetcher.Item) (*BatchSummary, error) {
	if len(items) == 0 {
		return &BatchSummary{}, nil
	}

	// Wait for rate limiter
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Build the prompt with all items
	var itemsList string
	for i, item := range items {
		itemsList += fmt.Sprintf("%d. Title: %s\n   Description: %s\n   Link: %s\n   ScrapedContent: %s\n\n", i+1, item.Title, item.Description, item.Link, item.ScrapedContent)
	}

	userPrompt := fmt.Sprintf("Source: %s\nNumber of items: %d\n\nItems:\n%s", sourceName, len(items), itemsList)

	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Info("retrying OpenAI batch request",
				slog.Int("attempt", attempt+1),
				slog.String("source", sourceName),
			)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(s.retryDelay * time.Duration(attempt+1)):
			}
		}

		summary, err := s.doBatchRequest(ctx, userPrompt)
		if err == nil {
			return summary, nil
		}
		lastErr = err
		s.logger.Warn("OpenAI batch request failed",
			slog.String("error", err.Error()),
			slog.Int("attempt", attempt+1),
		)
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}

func (s *OpenAISummarizer) doBatchRequest(ctx context.Context, userPrompt string) (*BatchSummary, error) {
	reqBody := openAIRequest{
		Model: s.model,
		Messages: []openAIMessage{
			{Role: "system", Content: BatchPrompt},
			{Role: "user", Content: userPrompt},
		},
		MaxTokens:   s.maxTokens,
		Temperature: 0.3,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if openAIResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", openAIResp.Error.Message)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &BatchSummary{Overview: openAIResp.Choices[0].Message.Content}, nil
}
