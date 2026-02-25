# TickStream Engine - Real-Time Trading Data Streaming

## Architecture

```
Exchanges (WebSocket) → Python Producers → Kafka → Flink (Java 21) → TimescaleDB + Kafka Output
                                                                              ↓
                                                                   Prometheus + Grafana
```

**Objective:** Anomaly detection in cryptocurrency prices at 2,000 msg/s with persistence.

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
| Flink Metrics | 9250 | ⏳ PENDING |
## Key Files

- `tickstream/` - Java Spring Boot + Flink project
- `src/producers/` - Python Kafka producers with Prometheus metrics
- `config/prometheus.yml` - Monitoring configuration
- `config/grafana-dashboard.json` - Metrics dashboard
- `db/schema.sql` - TimescaleDB schema
