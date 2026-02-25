"""
Coinbase WebSocket Producer.
Conecta al Advanced Trade API de Coinbase.
"""

import asyncio
import json
import logging
from typing import Optional

import websockets

from .base import BaseProducer


logger = logging.getLogger(__name__)


PRODUCT_IDS = ["BTC-USD", "ETH-USD", "SOL-USD", "DOGE-USD", "ADA-USD"]
WS_URL = "wss://advanced-trade-ws.coinbase.com"


class CoinbaseProducer(BaseProducer):
    def __init__(
        self,
        bootstrap_servers: list[str],
        topic: str = "trades-raw",
        product_ids: list[str] = PRODUCT_IDS,
        max_rate_per_second: int = None,
    ):
        super().__init__(
            "coinbase",
            bootstrap_servers,
            topic,
            max_rate_per_second=max_rate_per_second,
        )
        self.product_ids = product_ids
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
                        "type": "subscribe",
                        "product_ids": self.product_ids,
                        "channel": "market_trades",
                    }
                    await ws.send(json.dumps(subscribe_msg))
                    logger.info(f"[{self.name}] Subscribed to: {self.product_ids}")

                    while self._running:
                        try:
                            message = await ws.recv()
                            await self._process_message(message)
                        except websockets.exceptions.ConnectionClosed:
                            logger.warning(f"[{self.name}] Connection closed")
                            break

            except Exception as e:
                logger.error(f"[{self.name}] Stream error: {e}")

            await asyncio.sleep(reconnect_delay)
            reconnect_delay = min(reconnect_delay * 2, max_reconnect_delay)
            logger.info(f"[{self.name}] Reconnecting in {reconnect_delay}s...")

    async def _process_message(self, message: str):
        try:
            data = json.loads(message)

            channel = data.get("channel")
            if channel != "market_trades":
                return

            for event in data.get("events", []):
                for trade in event.get("trades", []):
                    normalized = {
                        "exchange": "coinbase",
                        "symbol": trade["product_id"].replace("-", ""),
                        "price": float(trade["price"]),
                        "qty": float(trade["size"]),
                        "ts": int(self._parse_timestamp(trade["time"])),
                        "trade_id": trade["trade_id"],
                        "side": trade.get("side", "unknown"),
                    }
                    self.send(normalized, key=normalized["symbol"])

        except json.JSONDecodeError as e:
            logger.error(f"[{self.name}] JSON decode error: {e}")
        except Exception as e:
            logger.error(f"[{self.name}] Process error: {e}")

    def _parse_timestamp(self, ts: str) -> int:
        try:
            if ts.isdigit():
                return int(ts)
            from datetime import datetime

            dt = ts.replace("Z", "+00:00")
            if "." in dt:
                dt = dt.split(".")[0] + "+00:00"
            return int(datetime.fromisoformat(dt).timestamp() * 1000)
        except Exception:
            import time

            return int(time.time() * 1000)


async def run():
    producer = CoinbaseProducer(["localhost:9092"])
    await producer.start()
    try:
        await producer.stream()
    finally:
        await producer.stop()


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    asyncio.run(run())
