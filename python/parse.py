from datetime import datetime
import feedparser
from dateutil import parser
import logging
from typing import List

from config import FeedConfig
from display import display_time
from dataclasses import dataclass, field
from typing import List

@dataclass
class Feed:
    source: str
    title: str
    summary: str
    link: str
    publish_date: str
    keywords: List[str] = field(default_factory=list)

    def __str__(self):
        res = f"Source: {self.source} \n"
        res += f"Title: {self.title} \n"
        if len(self.summary) > 0:
            res += f"Summary: {self.summary} \n"
        res += f"Published At: {self.publish_date} \n"
        res += f"\n{self.link}"

        return res


def parse(src: str, content: str, since: datetime, config: FeedConfig, logger: logging.Logger) -> List[Feed]:
    """
    parse takes an `src` and a `content` and returns a 
    `Feed` object.
    """
    feed = feedparser.parse(content)

    entries = feed.get("entries", {})
    logger.info(f"source: {src}, {len(entries)=}, feed keys: {feed.keys()}")

    feeds: List[Feed] = [] 
    for entry in reversed(entries):
        try:
            publish_date = parser.parse(entry.get("published", ""), ignoretz=True)
        except parser.ParserError as e:
            logger.error(f"failed to parse date: {e}")
            continue

        # skip old entries
        if publish_date < since:
            logger.info(f"article was too old: {publish_date=}, {since=}")
            continue
            

        title = entry.get("title", "")
        if len(title) > config.title_char_limit:
            title = title[:config.title_char_limit] + "..."

        summary = entry.get("summary", "")
        if len(summary) > config.summary_char_limit:
            summary = summary[:config.summary_char_limit] + "..."

        link = entry.get("link", "")

        feeds.append(Feed(
            source=src,
            title=title,
            summary=summary,
            link=link,
            publish_date=display_time(publish_date),
        ))
    
    return feeds