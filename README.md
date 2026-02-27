# TickStream — Real-Time Price Anomaly Detection

## Architecture

```
Exchanges (WebSocket) → Python Producers → Kafka → Flink (Java 21) → TimescaleDB + Kafka Output
                                                                              ↓
                                                                   Prometheus + Grafana
```

**Objective:** Anomaly detection in cryptocurrency prices at 200 msg/s with persistence.

## Technology Stack

| Component | Technology | Version |
|-----------|------------|---------|
| Streaming | Apache Kafka | 3.7.x |
| Processing | Apache Flink | 2.2.0 |
| Language | Java | 21 |
| Database | TimescaleDB (PostgreSQL) | 15.x |
| Framework | Spring Boot | 3.5.11 |
| Build | Gradle | 8.14.4 |
| Metrics | Prometheus | 2.50.x |
| Visualization | Grafana | 11.x |
| Producers | Python | 3.11.x |

## Quick Start

```bash
# 1. Start infrastructure
# Kafka, TimescaleDB, Prometheus, Grafana

# 2. Start Python producers (port 9091 for metrics)
cd src/producers && python src\producers\main.py

# 3. Start Flink job (Java 21)
cd tickstream && ./gradlew bootRun
```

## Ports
| Service | Port | Status |
|---------|------|--------|
| Kafka | 9092 | ❌ CLOSED |
| TimescaleDB | 5432 | ❌ CLOSED |
| Prometheus | 9090 | 🔍 VERIFY |
| Grafana | 3000 | 🔍 VERIFY |
| Producer Metrics | 9091 | ❌ CLOSED |
| Postgres Exporter | 9187 | 🔍 VERIFY |
| Kafka Exporter | 9308 |  🔍 VERIFY |
| Flink Metrics | 9250 | ⏳ EVALUATE |
## Key Files

- `tickstream/` - Java Spring Boot + Flink project Real Time
- `src/producers/` - Python Kafka producers with Prometheus metrics
- `config/prometheus.yml` - Monitoring configuration
- `config/grafana-dashboard.json` - Metrics dashboard 
- `db/schema.sql` - TimescaleDB schema

## TickStream · Observability
- Screenshot + JSON 
[![grafana.png](https://i.postimg.cc/tCpR8rn7/grafana.png)](https://postimg.cc/YjDwWz3w)

**Comentarios** 
- Threshold de anomalía: `|z| > 0.1` — permisivo intencionalmente para validar el pipeline completo durante desarrollo, actualizar a `|z| > 2.5`.
- El rango intradía observado por ventana de 1 minuto es de $8–$77 USD, con una desviación estándar típica de ~$10–$15, en el nivel de precio actual de BTCUSD.
- El pipeline opera actualmente a ~60-80 msg/s agregados (binance + coinbase + kraken).
  La arquitectura soporta escalar horizontalmente añadiendo particiones Kafka y 
  paralelismo Flink, sin cambios en el código de producción.
- End-to-end througput, la brecha para Binance con picos de ~160 msg/s pero con caídas periodicas ~20 msg/s en flujo de mensajes limitado por el rate-limit y/o reconexiones. Coinbase y Kraken tinen  throughpu bajo y estable.
- Producer Errors en 0, es relativamente bueno porque no hay fallas en el envío de mensajes al topic de entrada en Kafka.
- La latencia p50/p95/p99 colapsa en  sola línea cerca de 50ms indica que no hay tail latency significativa, lo cual es una señal sólida de que Kafka está bien dimensionado para el volumen actual.
