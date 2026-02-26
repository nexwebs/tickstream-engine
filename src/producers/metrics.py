"""
Métricas Prometheus para productores.
Expone endpoint /metrics en puerto 9091
"""

import logging
import time
import uuid
from prometheus_client import (
    start_http_server,
    Counter,
    Gauge,
    Histogram,
    CollectorRegistry,
)

logger = logging.getLogger(__name__)

registry = CollectorRegistry()

messages_sent = Counter(
    "tickstream_producer_messages_sent_total",
    "Total messages sent",
    ["exchange", "symbol"],
    registry=registry,
)

messages_failed = Counter(
    "tickstream_producer_messages_failed_total",
    "Total messages failed",
    ["exchange"],
    registry=registry,
)

producer_latency = Histogram(
    "tickstream_producer_send_latency_seconds",
    "Producer send latency in seconds",
    ["exchange"],
    buckets=[0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1],
    registry=registry,
)

active_producers = Gauge(
    "tickstream_producer_active",
    "Number of active producers",
    ["exchange"],
    registry=registry,
)

rate_limit_hits = Counter(
    "tickstream_producer_rate_limit_hits_total",
    "Rate limit hits",
    ["exchange"],
    registry=registry,
)

# Métrica de sesión activa
session_info = Gauge(
    "tickstream_producer_session_info",
    "Session active timestamp",
    ["session_id", "exchange"],
    registry=registry,
)

session_uptime = Gauge(
    "tickstream_producer_uptime_seconds",
    "Seconds since producer session started",
    ["session_id", "exchange"],
    registry=registry,
)

_session_id = str(uuid.uuid4())[:8]
_session_start = time.time()


def start_metrics_server(port=9091):
    """Inicia servidor de métricas Prometheus"""
    try:
        start_http_server(port, registry=registry)
        logger.info(f"Prometheus metrics server started on port {port}")
    except Exception as e:
        logger.error(f"Failed to start metrics server: {e}")


def record_send(exchange: str, symbol: str, latency_seconds: float):
    """Registra mensaje enviado exitosamente"""
    messages_sent.labels(exchange=exchange, symbol=symbol).inc()
    producer_latency.labels(exchange=exchange).observe(latency_seconds)


def record_failure(exchange: str):
    """Registra fallo de envío"""
    messages_failed.labels(exchange=exchange).inc()


def set_active(exchange: str, active: bool):
    """Marca producer como activo/inactivo"""
    active_producers.labels(exchange=exchange).set(1 if active else 0)


def record_rate_limit(exchange: str):
    """Registra rate limit hit"""
    rate_limit_hits.labels(exchange=exchange).inc()


def update_session():
    """Actualiza la métrica de sesión activa con timestamp actual y uptime"""
    elapsed = time.time() - _session_start
    for exchange in ["binance", "coinbase", "kraken"]:
        session_info.labels(session_id=_session_id, exchange=exchange).set(time.time())
        session_uptime.labels(session_id=_session_id, exchange=exchange).set(elapsed)
