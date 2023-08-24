from abc import ABC, abstractmethod
import requests

from config import TelegramProxyConfig

class Sender(ABC):
    @abstractmethod
    def send(self, msg: str) -> None:
        ...

class TelegramBotSender(Sender):
    def __init__(
            self,
            token: str,
            parse_mode: str,
            chan_id: str,
            proxy: TelegramProxyConfig,
    ) -> None:
        self.token = token
        self.parse_mode = parse_mode
        self.chan_id = chan_id
        self._session = requests.Session()
        if proxy.enabled:
            self._session.proxies = {
                "http": proxy.socks_proxy,
                "https": proxy.socks_proxy,
            }

    def send(self, msg: str) -> None:
        params = {
            "chat_id": self.chan_id,
            "text": msg,
            "parse_mode": self.parse_mode,
        }
        self._session.post(f"https://api.telegram.org/bot{self.token}/sendMessage", params=params)


class StdOutSender(Sender):
    def send(self, msg: str) -> None:
        print()
        print("####################")
        print(msg)
        print("####################")
        print()
        