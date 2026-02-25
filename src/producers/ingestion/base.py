"""
Base Producer: Clase abstracta con resiliencia para stream de datos.
Patrón: Circuit Breaker + Retry Exponential Backoff + Logging estructurado.
"""

import asyncio
import json
import logging
import time
from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from enum import Enum
from typing import Callable, Optional

from kafka import KafkaProducer
from kafka.errors import KafkaError

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))
import metrics as prom_metrics


logger = logging.getLogger(__name__)


class CircuitState(Enum):
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"


@dataclass
class CircuitBreakerConfig:
    failure_threshold: int = 5
    recovery_timeout: float = 30.0
    half_open_max_calls: int = 3


@dataclass
class ProducerMetrics:
    messages_sent: int = 0
    messages_failed: int = 0
    total_latency_ms: float = 0.0
    circuit_state: CircuitState = CircuitState.CLOSED
    last_error: Optional[str] = None


class CircuitBreaker:
    def __init__(self, config: CircuitBreakerConfig):
        self.config = config
        self.failure_count = 0
        self.success_count = 0
        self.state = CircuitState.CLOSED
        self.last_failure_time: Optional[float] = None
        self.half_open_calls = 0

    def call(self, func: Callable, *args, **kwargs):
        if self.state == CircuitState.OPEN:
            if time.time() - self.last_failure_time >= self.config.recovery_timeout:
                self.state = CircuitState.HALF_OPEN
                self.half_open_calls = 0
                logger.warning("Circuit breaker: transitioning to HALF_OPEN")
            else:
                raise CircuitBreakerOpenError("Circuit breaker is OPEN")

        try:
            result = func(*args, **kwargs)
            self._on_success()
            return result
        except Exception as e:
            self._on_failure()
            raise

    def _on_success(self):
        self.failure_count = 0
        if self.state == CircuitState.HALF_OPEN:
            self.success_count += 1
            if self.success_count >= self.config.half_open_max_calls:
                self.state = CircuitState.CLOSED
                self.success_count = 0
                logger.info("Circuit breaker: CLOSED")

    def _on_failure(self):
        self.failure_count += 1
        self.last_failure_time = time.time()
        if self.state == CircuitState.HALF_OPEN:
            self.state = CircuitState.OPEN
            logger.error("Circuit breaker: OPEN (half_open failed)")
        elif self.failure_count >= self.config.failure_threshold:
            self.state = CircuitState.OPEN
            logger.error(f"Circuit breaker: OPEN (failures: {self.failure_count})")


class CircuitBreakerOpenError(Exception):
    pass


class BaseProducer(ABC):
    def __init__(
        self,
        name: str,
        bootstrap_servers: list[str],
        topic: str,
        circuit_config: CircuitBreakerConfig = None,
        max_rate_per_second: int = None,  # NEW: Rate limiting
    ):
        self.name = name
        self.topic = topic
        self.circuit_breaker = CircuitBreaker(circuit_config or CircuitBreakerConfig())
        self.metrics = ProducerMetrics()
        self._producer: Optional[KafkaProducer] = None
        self._bootstrap_servers = bootstrap_servers
        self._running = False
        self._max_rate = max_rate_per_second
        self._messages_this_second = 0
        self._last_rate_reset = time.time()

    def _create_producer(self) -> KafkaProducer:
        return KafkaProducer(
            bootstrap_servers=self._bootstrap_servers,
            client_id="ticklevel-producer",
            value_serializer=lambda v: json.dumps(v).encode("utf-8"),
            key_serializer=lambda k: k.encode("utf-8") if k else None,
            compression_type="gzip",
            batch_size=131072,
            linger_ms=10,
            acks=1,
            retries=3,
            retry_backoff_ms=1000,
            enable_idempotence=False,
            max_in_flight_requests_per_connection=5,
            api_version_auto_timeout_ms=10000,
            connections_max_idle_ms=600000,
        )

    def _ensure_producer(self):
        if self._producer is None:
            self._producer = self._create_producer()
            logger.info(f"[{self.name}] Kafka producer initialized")

    async def start(self):
        self._ensure_producer()
        self._running = True
        logger.info(f"[{self.name}] Starting producer...")

    async def stop(self):
        self._running = False
        if self._producer:
            self._producer.flush()
            self._producer.close()
            self._producer = None
            logger.info(f"[{self.name}] Producer stopped")

    def send(self, data: dict, key: Optional[str] = None) -> bool:
        # Rate limiting
        if self._max_rate:
            now = time.time()
            if now - self._last_rate_reset >= 1.0:
                self._messages_this_second = 0
                self._last_rate_reset = now

            if self._messages_this_second >= self._max_rate:
                self.metrics.messages_failed += 1
                prom_metrics.record_failure(self.name)
                return False

            self._messages_this_second += 1

        start_time = time.time()

        def _send():
            future = self._producer.send(self.topic, key=key, value=data)
            future.get(timeout=10)
            return True

        try:
            self.circuit_breaker.call(_send)
            latency = (time.time() - start_time) * 1000
            self.metrics.messages_sent += 1
            self.metrics.total_latency_ms += latency

            # Record to Prometheus
            symbol = data.get("symbol", "UNKNOWN")
            prom_metrics.record_send(self.name, symbol, latency / 1000.0)

            logger.debug(
                f"[{self.name}] Sent: {data.get('symbol')} | Latency: {latency:.2f}ms"
            )
            return True

        except CircuitBreakerOpenError:
            logger.warning(f"[{self.name}] Circuit breaker OPEN - dropping message")
            self.metrics.messages_failed += 1
            prom_metrics.record_failure(self.name)
            return False
        except (KafkaError, Exception) as e:
            logger.error(f"[{self.name}] Send failed: {e}")
            self.metrics.messages_failed += 1
            self.metrics.last_error = str(e)
            prom_metrics.record_failure(self.name)
            return False

    @abstractmethod
    async def stream(self):
        pass

    def get_metrics(self) -> dict:
        avg_latency = (
            self.metrics.total_latency_ms / self.metrics.messages_sent
            if self.metrics.messages_sent > 0
            else 0.0
        )
        return {
            "producer": self.name,
            "messages_sent": self.metrics.messages_sent,
            "messages_failed": self.metrics.messages_failed,
            "avg_latency_ms": round(avg_latency, 2),
            "circuit_state": self.metrics.circuit_state.value,
        }
