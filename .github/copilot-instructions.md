# Copilot Instructions for Feed Parser

## Project Overview
RSS/Atom feed aggregator that fetches feeds from tech blogs and broadcasts them to Telegram channels. Designed to run as a scheduled cron job on a remote server. Available in both Python and Go implementations.

## Architecture

```
config.json          # Root-level configuration (sources, telegram, feed settings)
python/              # Python implementation
├── main.py          # Entry point: orchestrates fetch→parse→broadcast flow
├── config.py        # Dataclasses + JSON config loader
├── parse.py         # Feed parsing with feedparser lib + Feed dataclass
├── sender.py        # Abstract Sender + TelegramBotSender/StdOutSender
├── display.py       # Date formatting utility
go/                  # Go implementation
├── cmd/feedparser/main.go    # Entry point with CLI flags
├── internal/
│   ├── config/config.go      # Config structs + JSON loader
│   ├── feed/parser.go        # Feed parsing with gofeed lib
│   ├── sender/               # Sender interface + implementations
│   ├── summarizer/           # AI summarization (OpenAI, Gemini)
│   ├── scraper/              # HTML content extraction
│   └── cache/                # File-based summary caching
├── Makefile                  # Build automation
```

**Data Flow**: Load config → Fetch RSS → Filter by date/language → (Optional: Scrape article → AI summarize) → Broadcast to Telegram

## Key Patterns

### Configuration
- Config lives at `../config.json` relative to `python/` or `go/`
- Use dataclasses (Python) or structs (Go) for config types
- JSON keys differ from struct fields (e.g., `bot_access_token` → `bot_token`)

### Sender Pattern (Strategy)
Both implementations use the same pattern. New output channels implement:

**Python** (`sender.py`):
```python
class Sender(ABC):
    @abstractmethod
    def send(self, msg: str) -> None: ...
```

**Go** (`internal/sender/sender.go`):
```go
type Sender interface {
    Send(ctx context.Context, msg string) error
}
```

### Summarizer Pattern (Go only)
AI summarization uses the same interface pattern:
```go
type Summarizer interface {
    Summarize(ctx context.Context, title, content, articleURL string) (*Summary, error)
}
```
Implementations: `OpenAISummarizer`, `GeminiSummarizer`, `NoopSummarizer`

### Feed Filtering
- Entries filtered by `since_days_before` config value
- English-only filtering via `langdetect` (Python) / placeholder (Go)
- Special case: "The Go Blog" has 2-day grace period

### Message Format
Telegram messages use HTML parse mode:
```html
<b>{title}</b>
{source}
{publish_date}
<b>TLDR:</b> {ai_summary.tldr}
{ai_summary.full_summary}
<b>Links:</b>
• <a href="url">text</a>
{article_link}
```

## AI Summarization (Go only)

### Configuration
```json
"ai": {
    "enabled": true,
    "provider": "gemini",        // or "openai"
    "model": "gemini-1.5-flash", // or "gpt-4o-mini"
    "api_key": "YOUR_API_KEY",
    "max_tokens": 1024,
    "cache_enabled": true,
    "cache_dir": ".cache/summaries",
    "max_retries": 3,
    "retry_delay_ms": 1000,
    "rate_limit_rpm": 60
}
```

### Summary Output
- **TLDR**: Single paragraph key insight
- **Full Summary**: Multi-paragraph Markdown with technical details
- **Extracted Links**: Important references from the article
- **Code Snippets**: Relevant code examples with context

### Fallback Behavior
1. Try scraping full article HTML
2. Fall back to RSS content if scraping fails
3. Fall back to truncated summary if AI fails
4. Cache successful summaries by article URL (SHA256 hash)

## Development Commands

### Python
```bash
cd python
pip install -r requirements.txt
python3 main.py
```

### Go
```bash
cd go
make build      # Build binary to bin/feedparser
make run        # Build and run with default config
make run-stdout # Run with stdout output (debugging)
make run-debug  # Run with debug logging + stdout
make help       # Show all available targets
```

**Go CLI Flags:**
- `-c, --config`: Config file path (default: `../config.json`)
- `-l, --log-level`: Log level: debug, info, warn, error (default: `info`)
- `-f, --log-file`: Log file path (default: `feedparser.log`)
- `--stdout`: Also print messages to stdout

## Telegram Configuration

### Bot Setup
1. Create a bot via [@BotFather](https://t.me/BotFather) on Telegram
2. Copy the bot access token to `config.json` → `telegram.bot_access_token`
3. Create a channel and add the bot as admin with "Post Messages" permission
4. Set `telegram.channel_id` to the channel username (e.g., `@mychannel`) or numeric ID

### Proxy Configuration
For restricted networks, enable SOCKS5 proxy in `config.json`:
```json
"telegram": {
    "bot_access_token": "123456:ABC-DEF...",
    "channel_id": "@my_channel",
    "proxy": {
        "enabled": true,
        "socks_proxy": "socks5://127.0.0.1:56789"
    }
}
```
The `TelegramBotSender` in `sender.py` applies proxy to both HTTP/HTTPS requests via `requests.Session()`.

### Rate Limiting
Telegram has rate limits. Control sending speed via `config.json`:
- `wait_per_entry_seconds`: Delay between each message (default: 0.1s)
- `wait_per_source_seconds`: Delay between processing each feed source (default: 0.5s)

## Server Deployment

### Cron Job Setup
Run daily at 6 AM (matches `since_days_before: 1`):
```bash
crontab -e
# Add this line:
0 6 * * * cd /path/to/feedparser/python && python3 main.py
```

**Critical**: Cron interval must match `since_days_before` to avoid duplicates or missed posts.

### Logging
- Logs written to `python/file.log` and stdout
- Format: `%(asctime)s - %(levelname)s - %(message)s`
- Errors include exception type and message for debugging

## Adding New Feed Sources
Edit `config.json` → `sources` object:
```json
"sources": {
    "SourceName": "https://example.com/rss.xml"
}
```

## Adding New Output Channels

### Python
1. Create class implementing `Sender` in `sender.py`
2. Add config dataclass in `config.py` if needed
3. Instantiate in `main.py` and append to `senders` list

### Go
1. Create type implementing `Sender` interface in `internal/sender/`
2. Add config structs in `internal/config/config.go` if needed
3. Instantiate in `cmd/feedparser/main.go` and append to `senders` slice

## Common Issues
- **Duplicate posts**: Ensure cron interval matches `since_days_before`
- **Rate limits**: Increase `wait_per_entry_seconds` if Telegram blocks requests
- **Non-English posts**: Filtered automatically; adjust `is_english()` in `parse.py` if needed
- **Date parsing failures**: Uses `dateutil.parser` with `ignoretz=True`; check feed date format
