"""
Binance WebSocket Producer.
Conecta al stream de trades de Binance y envía a Kafka.
"""

import asyncio
import json
import logging
from typing import Optional

import websockets

from .base import BaseProducer


logger = logging.getLogger(__name__)


SYMBOLS = [
    "btcusdt",
    "ethusdt",
    "solusdt",
    "bnbusdt",
    "xrpusdt",
    "dogeusdt",
    "adausdt",
    "avaxusdt",
    "dotusdt",
    "maticusdt",
]
STREAM_URL = "stream.binance.com:9443"


class BinanceProducer(BaseProducer):
    def __init__(
        self,
        bootstrap_servers: list[str],
        topic: str = "trades-raw",
        symbols: list[str] = SYMBOLS,
        max_rate_per_second: int = None,
    ):
        super().__init__(
            "binance", bootstrap_servers, topic, max_rate_per_second=max_rate_per_second
        )
        self.symbols = symbols
        self.ws: Optional[websockets.WebSocketClientProtocol] = None

    def _build_stream_url(self) -> str:
        streams = "/".join([f"{s}@trade" for s in self.symbols])
        return f"wss://{STREAM_URL}/stream?streams={streams}"

    async def stream(self):
        url = self._build_stream_url()
        reconnect_delay = 1
        max_reconnect_delay = 60

        while self._running:
            try:
                async with websockets.connect(url, ping_interval=20) as ws:
                    self.ws = ws
                    reconnect_delay = 1
                    logger.info(f"[{self.name}] Connected to Binance WS")

                    while self._running:
                        try:
                            message = await ws.recv()
                            await self._process_message(message)
                        except websockets.exceptions.ConnectionClosed:
                            logger.warning(
                                f"[{self.name}] Connection closed, reconnecting..."
                            )
                            break

            except websockets.exceptions.InvalidURI:
                logger.error(f"[{self.name}] Invalid WS URI")
                break
            except Exception as e:
                logger.error(f"[{self.name}] Stream error: {e}")

            await asyncio.sleep(reconnect_delay)
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)
            logger.info(f"[{self.name}] Reconnecting in {reconnect_delay}s...")

    async def _process_message(self, message: str):
        try:
            data = json.loads(message)
            if "data" not in data:
                return

            trade = data["data"]
            normalized = {
                "exchange": "binance",
                "symbol": trade["s"],
                "price": float(trade["p"]),
                "qty": float(trade["q"]),
                "ts": trade["T"],
                "trade_id": trade["t"],
                "side": trade["m"] and "sell" or "buy",
            }
            self.send(normalized, key=trade["s"])

        except json.JSONDecodeError as e:
            logger.error(f"[{self.name}] JSON decode error: {e}")
        except KeyError as e:
            logger.error(f"[{self.name}] Missing key: {e}")


async def run():
    producer = BinanceProducer(["localhost:9092"])
    await producer.start()
    try:
        await producer.stream()
    finally:
        await producer.stop()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run())
