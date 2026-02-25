"""
Ingestion package: Producers para WebSocket exchanges.
"""

from .base import BaseProducer, CircuitBreaker, CircuitBreakerConfig
from .binance import BinanceProducer
from .coinbase import CoinbaseProducer
from .kraken import KrakenProducer

__all__ = [
    "BaseProducer",
    "CircuitBreaker",
    "CircuitBreakerConfig",
    "BinanceProducer",
    "CoinbaseProducer",
    "KrakenProducer",
]
