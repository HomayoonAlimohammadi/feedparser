from datetime import datetime, timedelta
import feedparser
from dateutil import parser
import logging
from typing import List
from langdetect import detect

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
    logger.info(f"Parsing `{src}`, number of results: {len(entries)}")

    feeds: List[Feed] = [] 
    for entry in reversed(entries):
        try:
            publish_date = parser.parse(entry.get("published", ""), ignoretz=True)
        except parser.ParserError as e:
            logger.error(f"failed to parse date: {e}")
            continue

        title = entry.get("title", "")
        if len(title) > config.title_char_limit:
            title = title[:config.title_char_limit] + "..."

        # skip old entries 
        # The Go Blog has a bug that publishes a new article with the date of yesterday!
        if publish_date < since and src != "The Go Blog":
            logger.info(f"`{title}` was too old, published at: {display_time(publish_date)}")
            continue
        elif src == "The Go Blog":
            if publish_date < since - timedelta(days=2):
                logger.info(f"`{title}` was too old, published at: {display_time(publish_date)}")
                continue
            else:
                logger.info(f"The Go Blog `{title}` was a tricky publish! published at:{display_time(publish_date)}")
                
        summary = entry.get("summary", "")
        if len(summary) > config.summary_char_limit:
            summary = summary[:config.summary_char_limit] + "..."

        try:
            if not is_english(title):
                continue
        except LangDetectError as e:
            logger.info(f"failed to detect language for `{title}`:", e)
            continue

        link = entry.get("link", "")

        feeds.append(Feed(
            source=src,
            title=title,
            summary=summary,
            link=link,
            publish_date=display_time(publish_date),
        ))

    logger.info(f"Parsed {len(feeds)} up-to-date feeds from `{src}`")
    
    return feeds

def is_english(s: str) -> bool:
    try:
        lang = detect(s)
        print(s, ":", lang)
        return lang == "en"
    except Exception as e:
        raise LangDetectError(e)


class LangDetectError(Exception):
    ...
