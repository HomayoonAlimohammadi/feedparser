from datetime import datetime, timedelta
from time import sleep
import requests
from typing import List
import logging

from sender import Sender, StdOutSender, TelegramBotSender 
from config import load_config, Source, FeedConfig
from parse import Feed, parse
from display import display_time

def setup_logger() -> logging.Logger:
    logger = logging.getLogger(__name__)
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')

    f_handler = logging.FileHandler('file.log')
    f_handler.setLevel(logging.INFO)
    f_format = logging.Formatter('%(asctime)s - %(levelname)s - %(message)s')
    f_handler.setFormatter(f_format)

    logger.addHandler(f_handler)

    return logger

def broadcast(senders: List[Sender], f: Feed) -> None:
    msg_html = f"<b>{f.title}</b> \n"
    msg_html += "\n"
    msg_html += f"{f.source}\n"
    msg_html += f"{f.publish_date}\n"
    msg_html += f"\n{f.link}"
    for sender in senders:
        sender.send(msg_html)
    
def fetch_and_send(senders: List[Sender], sources: List[Source], feed_config: FeedConfig, since: datetime,
                   logger: logging.Logger):
    for source in sources:
            feeds: List[Feed] = []
            try:
                content = requests.get(source.feed_url)
                feeds = parse(source.name, content.text, since, feed_config, logger)
            except Exception as e:
                logger.error(f"failed to get feeds for `{source.name}`: {type(e)}, {e}")

            try:
                for f in feeds:
                    broadcast(senders, f)
                    sleep(feed_config.wait_per_entry_seconds)
            except Exception as e:
                logger.error(f"failed to broadcast:, {type(e)}, {e}")

            sleep(feed_config.wait_per_source_seconds)

def main():
    config = load_config("../config.json")
    logger = setup_logger()

    logger.info(f"started running at {display_time(datetime.now())}")

    senders: List[Sender] = []

    senders.append(TelegramBotSender(
        token=config.telegram_config.bot_token,
        chan_id=config.telegram_config.channel_id,
        parse_mode="HTML",
        proxy=config.telegram_config.proxy,
    ))

    # senders.append(StdOutSender())

    current_time = datetime.now()
    # because some articles publish in the middle of day X
    # but label as 00:00:00 of the same day
    # this might cause in duplicates
    since = current_time - timedelta(
        days=config.feed_config.since_days_before,
        hours=current_time.hour,
        minutes=current_time.minute,
        seconds=current_time.second,
    )

    try:
        fetch_and_send(
            senders=senders,
            feed_config=config.feed_config,
            since=since,
            sources=config.sources,
            logger=logger
        )
    except Exception as e:
        logger.error(f"failed to fetch and send:", type(e), e)

    logger.info(f"finished running at {display_time(datetime.now())}")
    

if __name__ == "__main__":
    main()
