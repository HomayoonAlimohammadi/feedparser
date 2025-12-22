package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"

	"github.com/HomayoonAlimohammadi/feedparser/internal/config"
	"github.com/HomayoonAlimohammadi/feedparser/internal/fetcher"
	"github.com/HomayoonAlimohammadi/feedparser/internal/scraper"
	"github.com/HomayoonAlimohammadi/feedparser/internal/sender"
	"github.com/HomayoonAlimohammadi/feedparser/internal/summarizer"
)

func main() {
	// Parse CLI flags
	configPath := pflag.StringP("config", "c", "../config.json", "path to config file")
	logLevel := pflag.StringP("log-level", "l", "info", "log level (debug, info, warn, error)")
	logFile := pflag.StringP("log-file", "f", "feedparser.log", "log file path (empty to disable file logging)")
	pflag.Parse()

	// Setup logger
	logger, cleanup, err := setupLogger(*logLevel, *logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	// Setup context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Run the application
	if err := run(ctx, logger, *configPath); err != nil {
		logger.Error("application error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func setupLogger(level, logFile string) (*slog.Logger, func(), error) {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	writers := []io.Writer{os.Stdout}

	var file *os.File
	if logFile != "" {
		var err error
		file, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, nil, fmt.Errorf("opening log file: %w", err)
		}
		writers = append(writers, file)
	}

	multiWriter := io.MultiWriter(writers...)
	handler := slog.NewJSONHandler(multiWriter, opts)
	logger := slog.New(handler)

	cleanup := func() {
		if file != nil {
			file.Close()
		}
	}

	return logger, cleanup, nil
}

func run(ctx context.Context, logger *slog.Logger, configPath string) error {
	logger.Info("starting feedparser", slog.String("time", formatTime(time.Now())))

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger.Info("loaded configuration",
		slog.Int("sources", len(cfg.Feeds)),
		slog.Int("since_days_before", cfg.Feed.SinceDaysBefore),
		slog.Bool("ai_enabled", cfg.AI.Enabled),
		slog.Bool("hacker_news_enabled", cfg.HackerNews.Enabled),
		slog.Bool("reddit_enabled", cfg.Reddit.Enabled),
	)

	// Setup senders
	var senders []sender.Sender

	if cfg.Senders.Stdout.Enabled {
		senders = append(senders, sender.NewStdoutSender(logger))
	}

	if cfg.Senders.Telegram.Enabled {
		senders = append(senders, sender.NewTelegramSender(
			cfg.Senders.Telegram.BotToken,
			cfg.Senders.Telegram.ChannelID,
			logger,
		))
	}

	// Setup AI summarizer
	var sum summarizer.Summarizer
	if cfg.AI.Enabled {
		switch cfg.AI.Provider {
		case "openai":
			sum = summarizer.NewOpenAISummarizer(cfg.AI, logger)
			logger.Info("using OpenAI summarizer", slog.String("model", cfg.AI.Model))
		case "gemini":
			sum = summarizer.NewGeminiSummarizer(cfg.AI, logger)
			logger.Info("using Gemini summarizer", slog.String("model", cfg.AI.Model))
		default:
			logger.Warn("unknown AI provider, disabling AI summarization", slog.String("provider", cfg.AI.Provider))
			sum = summarizer.NewNoopSummarizer()
		}
	} else {
		sum = summarizer.NewNoopSummarizer()
	}

	// // Setup cache
	// var summaryCache cache.Cache
	// if cfg.AI.Enabled && cfg.AI.CacheEnabled {
	// 	summaryCache, err = cache.NewFileCache(cfg.AI.CacheDir, logger)
	// 	if err != nil {
	// 		logger.Warn("failed to create cache, disabling", slog.String("error", err.Error()))
	// 		summaryCache = cache.NewNoopCache()
	// 	}
	// } else {
	// 	summaryCache = cache.NewNoopCache()
	// }

	// Setup scraper
	scr := scraper.NewScraper(logger)

	// Calculate "since" time
	// Go back to the beginning of the day minus since_days_before
	now := time.Now()
	since := time.Date(
		now.Year(), now.Month(), now.Day(),
		0, 0, 0, 0, now.Location(),
	).AddDate(0, 0, -cfg.Feed.SinceDaysBefore)

	logger.Info("filtering entries since", slog.String("since", formatTime(since)))

	// Setup HTTP client
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Build list of all fetchers
	fetchers := make([]fetcher.Fetcher, 0)

	// Add RSS fetchers for each source
	for _, source := range cfg.Feeds {
		fetchers = append(fetchers, fetcher.NewRSSFetcher(source.Name, source.FeedURL, logger, scr))
	}

	// Add Hacker News fetcher if enabled
	if cfg.HackerNews.Enabled && len(cfg.HackerNews.Keywords) > 0 {
		hnFetcher := fetcher.NewHackerNewsFetcher(
			cfg.HackerNews.Keywords,
			httpClient,
			scr,
			logger,
			fetcher.WithMinPoints(cfg.HackerNews.MinPoints),
		)
		fetchers = append(fetchers, hnFetcher)
		logger.Info("added Hacker News fetcher",
			slog.Any("keywords", cfg.HackerNews.Keywords),
			slog.Int("min_points", cfg.HackerNews.MinPoints),
		)
	}

	// Add Reddit fetcher if enabled
	if cfg.Reddit.Enabled {
		opts := []fetcher.RedditOption{
			fetcher.WithMinScore(cfg.Reddit.MinScore),
		}
		if len(cfg.Reddit.Subreddits) > 0 {
			opts = append(opts, fetcher.WithSubreddits(cfg.Reddit.Subreddits))
		}
		if len(cfg.Reddit.Keywords) > 0 {
			opts = append(opts, fetcher.WithRedditKeywords(cfg.Reddit.Keywords))
		}
		redditFetcher := fetcher.NewRedditFetcher(httpClient, scr, logger, opts...)
		fetchers = append(fetchers, redditFetcher)
		logger.Info("added Reddit fetcher",
			slog.Any("subreddits", cfg.Reddit.Subreddits),
			slog.Any("keywords", cfg.Reddit.Keywords),
			slog.Int("min_score", cfg.Reddit.MinScore),
		)
	}

	// Fetch all items from all fetchers and group by source
	itemsBySource := make(map[string][]fetcher.Item)
	var allItems []fetcher.Item
	for _, f := range fetchers {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return nil
		default:
		}

		items, err := f.Fetch(ctx, since)
		if err != nil {
			logger.Error("failed to fetch items",
				slog.String("source", f.Name()),
				slog.String("error", err.Error()),
			)
			continue
		}

		logger.Info("fetched items",
			slog.String("source", f.Name()),
			slog.Int("count", len(items)),
		)

		// Group by source
		for _, item := range items {
			itemsBySource[item.Source] = append(itemsBySource[item.Source], item)
		}
		allItems = append(allItems, items...)
		// time.Sleep(cfg.Feed.WaitPerSource)
	}

	logger.Info("total items fetched", slog.Int("count", len(allItems)))

	if cfg.Digest.PerFetcher.Enabled {
		if err := processAndSendPerFetcherBatchItems(ctx, logger, itemsBySource, cfg, sum, senders); err != nil {
			return fmt.Errorf("failed to process and send single items: %w", err)
		}
	} else {
		logger.Info("per fetcher digest is disabled - skipping.")
	}

	if cfg.Digest.SingleItem.Enabled {
		if err := processAndSendSingleItems(ctx, logger, allItems, cfg, sum, senders); err != nil {
			return fmt.Errorf("failed to process and send single items: %w", err)
		}
	} else {
		logger.Info("single item digest is disabled - skipping.")
	}

	logger.Info("finished feedparser", slog.String("time", formatTime(time.Now())))
	return nil
}

func processAndSendPerFetcherBatchItems(
	ctx context.Context,
	logger *slog.Logger,
	itemsBySource map[string][]fetcher.Item,
	cfg *config.Config,
	sum summarizer.Summarizer,
	senders []sender.Sender,
) error {
	logger.Info("processing and sending per fetcher batched items", slog.Int("count_sources", len(itemsBySource)))

	digestBySource := make(map[string]string)

	// Generate and send per-fetcher digest summaries
	for sourceName, items := range itemsBySource {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return nil
		default:
		}

		if len(items) == 0 {
			logger.Info("source does not have any items",
				slog.String("source", sourceName))
			continue
		}

		var (
			batchSummary *summarizer.BatchSummary
			err          error
		)
		// Generate batch summary
		batchSummary, err = sum.SummarizeBatch(ctx, sourceName, items)
		if err != nil {
			logger.Warn("failed to generate batch summary",
				slog.String("source", sourceName),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Format and send the digest message
		digestBySource[sourceName] = formatPerFetcherDigestMessage(sourceName, items, batchSummary, cfg.Digest.PerFetcher.IncludeHeader)

		time.Sleep(cfg.Feed.WaitPerEntry)
	}

	digestMsg := formatAggregatedFetcherDigestMessage(digestBySource)

	for _, s := range senders {
		if err := s.Send(ctx, digestMsg); err != nil {
			logger.Error("failed to send digest message",
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	return nil
}

func processAndSendSingleItems(
	ctx context.Context,
	logger *slog.Logger,
	allItems []fetcher.Item,
	cfg *config.Config,
	sum summarizer.Summarizer,
	senders []sender.Sender,
) error {
	logger.Info("processing and sending single items",
		slog.Int("item_count", len(allItems)),
		slog.Int("count_senders", len(senders)),
	)

	// Process and send all items
	for _, item := range allItems {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return nil
		default:
		}

		var (
			aiSummary *summarizer.Summary
			err       error
		)
		if cfg.AI.Enabled {
			// Generate AI summary
			if item.ScrapedContent != "" {
				aiSummary, err = sum.Summarize(ctx, item)
				if err != nil {
					logger.Warn("failed to generate AI summary",
						slog.String("title", item.Title),
						slog.String("error", err.Error()),
					)

				}
			}
		}

		// Format message with score/comments info for HN/Reddit
		msg := formatSingleItemMessage(cfg, item, aiSummary)
		for _, s := range senders {
			if err := s.Send(ctx, msg); err != nil {
				logger.Error("failed to send message",
					slog.String("title", item.Title),
					slog.String("error", err.Error()),
				)
			}
		}

		logger.Info("sent item", slog.String("title", item.Title),
			slog.String("source", item.Source))

		time.Sleep(cfg.Feed.WaitPerEntry)
	}

	return nil
}

// // formatFetcherMessage creates the HTML message for items from fetchers (HN, Reddit).
// func formatFetcherMessage2(source, title, link string) string {
// 	if len(title) > 20 {
// 		title = title[:20]
// 	}
// 	return fmt.Sprintf("%s: %s... -- %s\n", source, title, link)
// }

// formatSingleItemMessage creates the HTML message for items from fetchers (HN, Reddit).
func formatSingleItemMessage(cfg *config.Config, item fetcher.Item, aiSummary *summarizer.Summary) string {
	var sb strings.Builder

	// Apply title limit
	if len(item.Title) > cfg.Feed.TitleCharLimit {
		item.Title = item.Title[:cfg.Feed.TitleCharLimit] + "..."
	}

	// Apply summary limit
	if len(item.Description) > cfg.Feed.SummaryCharLimit {
		item.Description = item.Description[:cfg.Feed.SummaryCharLimit] + "..."
	}

	if aiSummary != nil && len(aiSummary.TLDR) > cfg.Feed.SummaryCharLimit {
		aiSummary.TLDR = aiSummary.TLDR[:cfg.Feed.SummaryCharLimit] + "..."
	}

	// Title
	sb.WriteString(fmt.Sprintf("<b>%s</b>\n\n", item.Title))

	// Source, date, and engagement info
	sb.WriteString(fmt.Sprintf("%s\n%s\n", item.Source, item.PublishDate))
	if item.Score > 0 || item.Comments > 0 {
		sb.WriteString(fmt.Sprintf("⬆️ %d points | 💬 %d comments\n", item.Score, item.Comments))
	}

	// Keywords if any
	if len(item.Keywords) > 0 {
		sb.WriteString(fmt.Sprintf("🏷️ %s\n", strings.Join(item.Keywords, ", ")))
	}
	sb.WriteString("\n")

	// AI Summary or fallback
	if aiSummary != nil && aiSummary.TLDR != "" {
		sb.WriteString(fmt.Sprintf("<b>TLDR:</b> %s\n", aiSummary.TLDR))

		// // Add full summary if available (truncated for Telegram)
		// if f.AISummary.FullSummary != "" {
		// 	summary := f.AISummary.FullSummary
		// 	const maxSummaryLen = 2500
		// 	if len(summary) > maxSummaryLen {
		// 		summary = summary[:maxSummaryLen] + "..."
		// 	}
		// 	sb.WriteString(summary)
		// 	sb.WriteString("\n\n")
		// }

		// // Add extracted links
		// if len(f.AISummary.ExtractedLinks) > 0 {
		// 	sb.WriteString("<b>Links:</b>\n")
		// 	for _, link := range f.AISummary.ExtractedLinks {
		// 		if len(link.Text) > 50 {
		// 			link.Text = link.Text[:50] + "..."
		// 		}
		// 		sb.WriteString(fmt.Sprintf("• <a href=\"%s\">%s</a>\n", link.URL, link.Text))
		// 	}
		// 	sb.WriteString("\n")
		// }
	} else if item.Description != "" {
		sb.WriteString(fmt.Sprintf("<b>TLDR:</b> %s\n", item.Description))
		sb.WriteString("\n\n")
	}

	// Article link
	sb.WriteString(item.Link)

	return sb.String()
}

func formatAggregatedFetcherDigestMessage(digestBySource map[string]string) string {
	if len(digestBySource) < 1 {
		return ""
	}

	sb := &strings.Builder{}

	sb.WriteString("<h1>Blog Digest</h1>\n")
	fmt.Fprintf(sb, "Date: %s\n\n", time.Now().Format(time.RFC1123))

	for sourceName, digest := range digestBySource {
		fmt.Fprintf(sb, "<h2>%s</h2>\n", sourceName)
		sb.WriteString(digest)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// formatPerFetcherDigestMessage creates the HTML message for a per-fetcher digest summary.
func formatPerFetcherDigestMessage(sourceName string, items []fetcher.Item, aiSummary *summarizer.BatchSummary, includeHeader bool) string {
	if len(items) < 1 {
		if includeHeader {
			return fmt.Sprintf("%s: None", sourceName)
		} else {
			return "None"
		}
	}

	sb := &strings.Builder{}

	// Header
	if includeHeader {
		fmt.Fprintf(sb, "<b>%s</b>\n", sourceName)
	}

	// Overview
	if aiSummary != nil && aiSummary.Overview != "" {
		fmt.Fprintf(sb, "<b>Overview:</b>\n%s\n\n", aiSummary.Overview)
	} else {
		sb.WriteString("<b>Overview:</b> -\n\n")
	}

	for _, item := range items {
		fmt.Fprintf(sb, "- %s: %s\n", item.Title, item.Link)
	}

	return sb.String()
}

// formatTime formats a time.Time to a human-readable string.
// Matches the Python display_time format: "December 8th 2025 14:30:0"
func formatTime(t time.Time) string {
	months := []string{
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	day := t.Day()
	suffix := "th"
	if day < 11 || day > 13 {
		switch day % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}

	return fmt.Sprintf("%s %d%s %d %d:%02d:%d",
		months[t.Month()],
		day, suffix,
		t.Year(),
		t.Hour(), t.Minute(), t.Second(),
	)
}
