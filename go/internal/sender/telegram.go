package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// TelegramSender sends messages to a Telegram channel via Bot API.
type TelegramSender struct {
	token     string
	channelID string
	parseMode string
	client    *http.Client
	logger    *slog.Logger
}

// TelegramOption configures a TelegramSender.
type TelegramOption func(*TelegramSender)

// WithParseMode sets the parse mode (HTML, Markdown, etc.).
func WithParseMode(mode string) TelegramOption {
	return func(s *TelegramSender) {
		s.parseMode = mode
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) TelegramOption {
	return func(s *TelegramSender) {
		s.client = client
	}
}

// NewTelegramSender creates a new Telegram sender.
func NewTelegramSender(token, channelID string, logger *slog.Logger, opts ...TelegramOption) *TelegramSender {
	s := &TelegramSender{
		token:     token,
		channelID: channelID,
		parseMode: "HTML",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// telegramRequest represents the Telegram sendMessage request body.
type telegramRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// Send sends a message to the configured Telegram channel.
func (s *TelegramSender) Send(ctx context.Context, msg string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token)

	reqBody := telegramRequest{
		ChatID:    s.channelID,
		Text:      msg,
		ParseMode: s.parseMode,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	s.logger.Debug("message sent to Telegram",
		slog.String("channel", s.channelID),
	)

	return nil
}
