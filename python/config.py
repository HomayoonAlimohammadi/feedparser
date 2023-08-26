from dataclasses import dataclass, field
from typing import List
import json

from colorama import init

@dataclass
class FeedConfig:
    title_char_limit: int
    summary_char_limit: int
    since_days_before: int
    wait_per_entry_seconds: int
    wait_per_source_seconds: int

@dataclass
class TelegramProxyConfig:
    socks_proxy: str
    enabled: bool = field(default=False)

@dataclass
class TelegramConfig:
    bot_token: str
    channel_id: str
    proxy: TelegramProxyConfig

@dataclass
class Source:
    name: str
    feed_url: str

@dataclass
class Config:
    feed_config: FeedConfig
    telegram_config: TelegramConfig
    sources: List[Source] = field(default_factory=list)

def load_config(path: str) -> Config:
    with open(path, "r") as f:
        data = json.load(f)

    sources: List[Source] = []
    for name, url, in data.get("sources", {}).items():
        sources.append(Source(name=name, feed_url=url))

    feed = data.get("feed", {})
    feed_config = FeedConfig(
        summary_char_limit=feed.get("summary_char_limit", 200),
        title_char_limit=feed.get("title_char_limit", 300),
        since_days_before=feed.get("since_days_before", 30),
        wait_per_entry_seconds=feed.get("wait_per_entry_seconds", 3),
        wait_per_source_seconds=feed.get("wait_per_source_seconds", 5),
    )

    telegram = data.get("telegram", {})
    proxy = telegram.get("proxy", {})
    telegram_config = TelegramConfig(
        bot_token=telegram.get("bot_access_token", ""),
        channel_id=telegram.get("channel_id", ""),
        proxy=TelegramProxyConfig(
            enabled=proxy.get("enabled", False),
            socks_proxy=proxy.get("socks_proxy", ""),
        ),
    )

    return Config(
        feed_config=feed_config,
        telegram_config=telegram_config,
        sources=sources,
    ) 