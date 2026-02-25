"""
Kraken WebSocket Producer.
Conecta al WebSocket API de Kraken.
"""

import asyncio
import json
import logging
from typing import Optional

import websockets

from .base import BaseProducer


logger = logging.getLogger(__name__)


PAIRS = ["XBT/USD", "ETH/USD", "SOL/USD", "ADA/USD", "DOT/USD"]
WS_URL = "wss://ws.kraken.com"


class KrakenProducer(BaseProducer):
    def __init__(
        self,
        bootstrap_servers: list[str],
        topic: str = "trades-raw",
        pairs: list[str] = PAIRS,
        max_rate_per_second: int = None,
    ):
        super().__init__(
            "kraken", bootstrap_servers, topic, max_rate_per_second=max_rate_per_second
        )
        self.pairs = pairs
        self.ws: Optional[websockets.WebSocketClientProtocol] = None

    async def stream(self):
        reconnect_delay = 1
        max_reconnect_delay = 60

        while self._running:
            try:
                async with websockets.connect(WS_URL, ping_interval=20) as ws:
                    self.ws = ws
                    reconnect_delay = 1

                    subscribe_msg = {
                        "event": "subscribe",
                        "pair": self.pairs,
                        "subscription": {"name": "trade"},
                    }
                    await ws.send(json.dumps(subscribe_msg))
                    logger.info(f"[{self.name}] Subscribed to: {self.pairs}")

                    pair_map = {
                        pair: pair.replace("/", "").replace("XBT", "BTC")
                        for pair in self.pairs
                    }

                    while self._running:
                        try:
                            message = await ws.recv()
                            await self._process_message(message, pair_map)
                        except websockets.exceptions.ConnectionClosed:
                            logger.warning(f"[{self.name}] Connection closed")
                            break

            except Exception as e:
                logger.error(f"[{self.name}] Stream error: {e}")

            await asyncio.sleep(reconnect_delay)
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)
            logger.info(f"[{self.name}] Reconnecting in {reconnect_delay}s...")

    async def _process_message(self, message: str, pair_map: dict):
        try:
            data = json.loads(message)

            if isinstance(data, list) and len(data) >= 4:
                channel_name = data[2] if len(data) > 2 else None
                if channel_name == "trade":
                    trades = data[1]
                    pair = data[3]
                    symbol = pair_map.get(pair, pair.replace("/", ""))

                    for trade in trades:
                        normalized = {
                            "exchange": "kraken",
                            "symbol": symbol,
                            "price": float(trade[0]),
                            "qty": float(trade[1]),
                            "ts": int(float(trade[2]) * 1000),
                            "trade_id": trade[3] if len(trade) > 3 else None,
                            "side": trade[3] if len(trade) > 3 else "unknown",
                        }
                        self.send(normalized, key=symbol)

        except json.JSONDecodeError as e:
            logger.error(f"[{self.name}] JSON decode error: {e}")
        except Exception as e:
            logger.error(f"[{self.name}] Process error: {e}")


async def run():
    producer = KrakenProducer(["localhost:9092"])
    await producer.start()
    try:
        await producer.stream()
    finally:
        await producer.stop()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run())
