"""
Main entry point: Ejecuta todos los producers en paralelo.
Usage: python main.py [max_rate_per_second]
       python main.py 1500  -> limita a 1500 msg/s total
"""

import asyncio
import logging
import sys
import os
from pathlib import Path
from typing import List
import threading

sys.path.insert(0, str(Path(__file__).parent))

from ingestion import BinanceProducer, CoinbaseProducer, KrakenProducer
from ingestion.base import BaseProducer
import metrics


logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)-8s | %(name)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


class StreamOrchestrator:
    def __init__(
        self,
        bootstrap_servers: List[str],
        topic: str = "trades-raw",
        max_rate_per_second: int = None,
    ):
        self.bootstrap_servers = bootstrap_servers
        self.topic = topic
        self.max_rate = max_rate_per_second
        self.producers: List[BaseProducer] = []
        self._running = False

    def setup_producers(self):
        rate_per_producer = self.max_rate // 3 if self.max_rate else None

        self.producers = [
            BinanceProducer(
                self.bootstrap_servers,
                self.topic,
                max_rate_per_second=rate_per_producer,
            ),
            CoinbaseProducer(
                self.bootstrap_servers,
                self.topic,
                max_rate_per_second=rate_per_producer,
            ),
            KrakenProducer(
                self.bootstrap_servers,
                self.topic,
                max_rate_per_second=rate_per_producer,
            ),
        ]
        logger.info(
            f"Configured {len(self.producers)} producers (max_rate={self.max_rate}/s total, {rate_per_producer}/s each)"
        )

    async def start_all(self):
        self._running = True
        for p in self.producers:
            await p.start()

        tasks = [asyncio.create_task(p.stream()) for p in self.producers]
        logger.info("All producers started")

        while self._running:
            await asyncio.sleep(30)
            self._log_metrics()

    async def stop_all(self):
        self._running = False
        logger.info("Stopping all producers...")
        for p in self.producers:
            await p.stop()
        logger.info("All producers stopped")

    def _log_metrics(self):
        for p in self.producers:
            m = p.get_metrics()
            logger.info(
                f"[{m['producer']}] sent={m['messages_sent']} "
                f"failed={m['messages_failed']} "
                f"latency={m['avg_latency_ms']}ms "
                f"circuit={m['circuit_state']}"
            )


async def main():
    # Start metrics server in background thread
    metrics_thread = threading.Thread(
        target=lambda: metrics.start_metrics_server(port=9091), daemon=True
    )
    metrics_thread.start()
    logger.info("Metrics server started on port 9091")

    bootstrap_servers = ["127.0.0.1:9092"]
    topic = "trades-raw"

    # Rate limit configurable: python main.py [max_rate]
    # Por defecto: 1500/s (límite seguro para 8GB RAM)
    max_rate = int(sys.argv[1]) if len(sys.argv) > 1 else 1500

    orchestrator = StreamOrchestrator(
        bootstrap_servers, topic, max_rate_per_second=max_rate
    )
    orchestrator.setup_producers()

    try:
        await orchestrator.start_all()
    except (KeyboardInterrupt, asyncio.CancelledError):
        await orchestrator.stop_all()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
