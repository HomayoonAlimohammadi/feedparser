package summarizer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/HomayoonAlimohammadi/feedparser/internal/config"
	"github.com/HomayoonAlimohammadi/feedparser/internal/fetcher"
)

// GeminiSummarizer implements Summarizer using Google's Gemini API.
type GeminiSummarizer struct {
	apiKey      string
	model       string
	maxTokens   int
	maxRetries  int
	retryDelay  time.Duration
	rateLimiter *rateLimiter
	client      *http.Client
	logger      *slog.Logger
}

// NewGeminiSummarizer creates a new Gemini summarizer.
func NewGeminiSummarizer(cfg config.AIConfig, logger *slog.Logger) *GeminiSummarizer {
	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiSummarizer{
		apiKey:      cfg.APIKey,
		model:       model,
		maxTokens:   cfg.MaxTokens,
		maxRetries:  cfg.MaxRetries,
		retryDelay:  time.Duration(cfg.RetryDelayMs) * time.Millisecond,
		rateLimiter: newRateLimiter(cfg.RateLimitRPM),
		client:      &http.Client{Timeout: 120 * time.Second},
		logger:      logger,
	}
}

// geminiRequest represents the Gemini API request body.
type geminiRequest struct {
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
}

// geminiResponse represents the Gemini API response.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

// Summarize implements the Summarizer interface.
func (s *GeminiSummarizer) Summarize(ctx context.Context, item fetcher.Item) (*Summary, error) {
	// Wait for rate limiter
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	userPrompt := fmt.Sprintf("Article Title: %s\nArticle URL: %s\n\nArticle Content:\n%s", item.Title, item.Link, item.ScrapedContent)

	var lastErr error
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Info("retrying Gemini request",
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
		s.logger.Warn("Gemini request failed",
			slog.String("error", err.Error()),
			slog.Int("attempt", attempt+1),
		)
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}

func (s *GeminiSummarizer) doRequest(ctx context.Context, userPrompt string) (*Summary, error) {
	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: DefaultPrompt}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userPrompt}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: s.maxTokens,
			Temperature:     0.3,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", s.model, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	summary := Summary{
		TLDR: geminiResp.Candidates[0].Content.Parts[0].Text,
	}

	// // Try to extract JSON from markdown code block if present
	// content = extractJSON(content)

	// if err := json.Unmarshal([]byte(content), &summary); err != nil {
	// 	// If JSON parsing fails, use raw content
	// 	summary = Summary{
	// 		TLDR: content,
	// 		// FullSummary: content,
	// 	}
	// 	s.logger.Warn("failed to parse JSON response, using raw content",
	// 		slog.String("error", err.Error()),
	// 	)
	// }

	return &summary, nil
}

// // extractJSON tries to extract JSON from markdown code blocks.
// func extractJSON(content string) string {
// 	// Try to find JSON in code block
// 	start := -1
// 	end := -1

// 	// Look for ```json or ``` followed by {
// 	markers := []string{"```json\n", "```\n{"}
// 	for _, marker := range markers {
// 		if idx := indexOf(content, marker); idx != -1 {
// 			if marker == "```json\n" {
// 				start = idx + len(marker)
// 			} else {
// 				start = idx + 4 // len("```\n")
// 			}
// 			break
// 		}
// 	}

// 	if start == -1 {
// 		return content
// 	}

// 	// Find closing ```
// 	if idx := indexOf(content[start:], "```"); idx != -1 {
// 		end = start + idx
// 	}

// 	if end == -1 {
// 		return content
// 	}

// 	return content[start:end]
// }

// func indexOf(s, substr string) int {
// 	for i := 0; i <= len(s)-len(substr); i++ {
// 		if s[i:i+len(substr)] == substr {
// 			return i
// 		}
// 	}
// 	return -1
// }

// SummarizeBatch generates a digest summary of multiple items from a source.
func (s *GeminiSummarizer) SummarizeBatch(ctx context.Context, sourceName string, items []fetcher.Item) (*BatchSummary, error) {
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
			s.logger.Info("retrying Gemini batch request",
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
		s.logger.Warn("Gemini batch request failed",
			slog.String("error", err.Error()),
			slog.Int("attempt", attempt+1),
		)
	}

	return nil, fmt.Errorf("all retries exhausted: %w", lastErr)
}

func (s *GeminiSummarizer) doBatchRequest(ctx context.Context, userPrompt string) (*BatchSummary, error) {
	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: BatchPrompt}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: userPrompt}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			MaxOutputTokens: s.maxTokens,
			Temperature:     0.3,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", s.model, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if geminiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	return &BatchSummary{Overview: geminiResp.Candidates[0].Content.Parts[0].Text}, nil
}
