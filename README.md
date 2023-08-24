# Feed Parser

Parse your desired feeds and broadcast to whatever media you want.

## How to use

Create and customize a `config.json` file to your likings and run the program.

## Python version
```bash
pip install -r requirements.txt
python3 main.py &
```
Notice the `&` instruction. This way you can leave this script running on a remote server and 
enjoy your desired feeds being broadcasted to your desired media (e.g. telegram channel)

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
        "initial_days_before": 10,
        "sleep_interval_seconds": 86400,
        "wait_per_entry_seconds": 3,
        "wait_per_source_seconds": 5
    }
}
```

Enjoy!