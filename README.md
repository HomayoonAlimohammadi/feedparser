# Feed Parser

Parse your desired feeds and broadcast to whatever media you want.

## How to use

Create and customize a `config.json` file to your likings and run the program.

## Python version
```bash
pip install -r requirements.txt
python3 main.py
```
You can periodicly run this script on a remote server (cronjobs) and enjoy all the feeds being broadcasted to your desired media (e.g. telegram channel)

To run the script every morning on 7 AM:
```bash
crontab -e
* 7 * * * user cd path/to/main.py && python3 main.py
```

## Sample config.json
```json
{
    "telegram": {
        "bot_access_token": "XXX",
        "channel_id": "XXX",
        "proxy": {
            "enabled": false,
            "socks_proxy": "socks5://127.0.0.1:56789"
        }
    },
    "sources": {
        "Cloudflare": "https://blog.cloudflare.com/rss"
    },
    "feed": {
        "title_char_limit": 200,
        "summary_char_limit": 300,
        "since_days_before": 1,
        "wait_per_entry_seconds": 3,
        "wait_per_source_seconds": 5
    }
}
```

`NOTICE`: cronjob interval and `since_days_before` should logically match otherwise you're gonna get duplicate posts or you might miss a few.

Enjoy!
