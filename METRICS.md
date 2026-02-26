# TickStream Metrics Reference

## Productores Python (Puerto 9091)

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `tickstream_producer_messages_sent_total` | Counter | Mensajes enviados por exchange | `sum(tickstream_producer_messages_sent_total) by (exchange)` |
| `tickstream_producer_messages_failed_total` | Counter | Mensajes fallidos por exchange | `sum(tickstream_producer_messages_failed_total) by (exchange)` |
| `tickstream_producer_send_latency_seconds` | Histogram | Latencia de envío a Kafka | `sum(rate(tickstream_producer_send_latency_seconds_sum[1m])) / sum(rate(tickstream_producer_send_latency_seconds_count[1m])) * 1000` |
- `sum(rate(tickstream_producer_messages_sent_total[1m])) by (exchange)` Throughput promedio por segundo, calculado usando los últimos 60 segundos, agrupado por exchange.
---

## API Go (Puerto 8080) - Prometheus Client

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `tickstream_api_requests_total` | Counter | Total requests por endpoint y método | `sum(tickstream_api_requests_total) by (endpoint, method)` |
| `tickstream_api_request_duration_seconds` | Histogram | Latencia de requests API | `histogram_quantile(0.95, rate(tickstream_api_request_duration_seconds_bucket[5m]))` |
| `tickstream_db_query_duration_seconds` | Histogram | Latencia de queries a DB | `histogram_quantile(0.95, rate(tickstream_db_query_duration_seconds_bucket[5m]))` |
| `tickstream_api_db_connections_active` | Gauge | Conexiones DB activas | `tickstream_api_db_connections_active` |

### Métricas Calculadas API

| Métrica | Descripción | Query PromQL |
|---------|-------------|--------------|
| **Requests/segundo** | Throughput API | `sum(rate(tickstream_api_requests_total[1m]))` |
| **Latencia p95** | Percentil 95 latencia API | `histogram_quantile(0.95, rate(tickstream_api_request_duration_seconds_bucket[5m]))` |
| **Latencia p99** | Percentil 99 latencia API | `histogram_quantile(0.99, rate(tickstream_api_request_duration_seconds_bucket[5m]))` |

---

## Kafka (Puerto 9308) - kafka_exporter

### Topic Metrics

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `kafka_topic_partitions` | Gauge | Número de particiones por topic | `kafka_topic_partitions{topic="trades-raw"}` |
| `kafka_topic_partition_current_offset` | Gauge | Offset actual por partición | `kafka_topic_partition_current_offset{topic="trades-raw"}` |
| `kafka_topic_partition_oldest_offset` | Gauge | Offset más antiguo | `kafka_topic_partition_oldest_offset{topic="trades-raw"}` |

### Consumer Group Metrics

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `kafka_consumergroup_current_offset` | Gauge | Offset actual del consumer group | `kafka_consumergroup_current_offset{group="flink-anomaly-detector"}` |
| `kafka_consumergroup_lag` | Gauge | Liferencia producer-consAG (dumidor) | `kafka_consumergroup_lag{group="flink-anomaly-detector"}` |
| `kafka_consumergroup_lag_sum` | Gauge | LAG total por topic | `sum(kafka_consumergroup_lag) by (group, topic)` |
| `kafka_consumergroup_members` | Gauge | Miembros activos del grupo | `kafka_consumergroup_members{group="flink-anomaly-detector"}` |

### Broker Metrics

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `kafka_topic_partition_under_replicated_partition` | Gauge | 1 si partición sub-replicada | `kafka_topic_partition_under_replicated_partition{topic="trades-raw"} == 1` |

---

## TimescaleDB / PostgreSQL (Puerto 9187) - postgres_exporter

### Database Stats

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `pg_stat_database_tup_inserted` | Counter | Filas insertadas totales | `sum(pg_stat_database_tup_inserted{datname="tickleveldb"})` |
| `pg_stat_database_tup_updated` | Counter | Filas actualizadas | `sum(pg_stat_database_tup_updated{datname="tickleveldb"})` |
| `pg_stat_database_tup_fetched` | Counter | Filas leídas | `sum(pg_stat_database_tup_fetched{datname="tickleveldb"})` |
| `pg_stat_database_tup_returned` | Counter | Filas devueltas por queries | `sum(pg_stat_database_tup_returned{datname="tickleveldb"})` |
| `pg_stat_database_numbackends` | Gauge | Conexiones activas | `pg_stat_database_numbackends{datname="tickleveldb"}` |
| `pg_stat_database_blks_hit` | Counter | Bloques leídos de cache | `rate(pg_stat_database_blks_hit{datname="tickleveldb"}[1m])` |
| `pg_stat_database_blks_read` | Counter | Bloques leídos del disco | `rate(pg_stat_database_blks_read{datname="tickleveldb"}[1m])` |



### Connection & Size

| Métrica | Tipo | Descripción | Query PromQL |
|---------|------|-------------|---------------|
| `pg_database_size_bytes` | Gauge | Tamaño de la base de datos | `pg_database_size_bytes{datname="tickleveldb"}` |
| `pg_stat_database_conflict` | Counter | Conflictos de replicación | `sum(pg_stat_database_conflict{datname="tickleveldb"})` |
| `pg_stat_database_temp_files` | Counter | Archivos temporales creados | `sum(pg_stat_database_temp_files{datname="tickleveldb"})` |
| `pg_stat_database_deadlocks` | Counter | Deadlocks ocurridos | `sum(pg_stat_database_deadlocks{datname="tickleveldb"})` |

---

## Métricas Calculadas (Derivadas)

| Métrica | Descripción | Query PromQL |
|---------|-------------|--------------|
| **Throughput Productores** | Mensajes/segundo enviados | `sum(rate(tickstream_producer_messages_sent_total[1m]))` |
| **Tasa de Error Productores** | % mensajes fallidos | `sum(rate(tickstream_producer_messages_failed_total[5m])) / sum(rate(tickstream_producer_messages_sent_total[5m])) * 100` |
| **Kafka Consumer Lag** | LAG total Flink | `sum(kafka_consumergroup_lag{group="flink-anomaly-detector"})` |
| **Insert Rate DB** | Filas insertadas/segundo | `rate(pg_stat_database_tup_inserted{datname="tickleveldb"}[1m])` |
| **Cache Hit Ratio** | % cache hit PostgreSQL | `rate(pg_stat_database_blks_hit[1m]) / (rate(pg_stat_database_blks_hit[1m]) + rate(pg_stat_database_blks_read[1m])) * 100` |

---

## Queries Útiles para Grafana

### Dashboard Principal

```bash
# 1. Mensajes por segundo (producers)
sum(rate(tickstream_producer_messages_sent_total[1m]))

# 2. Latencia promedio envío (ms)
(sum(rate(tickstream_producer_send_latency_seconds_sum[1m])) / sum(rate(tickstream_producer_send_latency_seconds_count[1m]))) * 1000

# 3. Kafka LAG total
sum(kafka_consumergroup_lag{group="flink-anomaly-detector"})

# 4. Filas insertadas en DB (último minuto)
rate(pg_stat_database_tup_inserted{datname="tickleveldb"}[1m])

# 5. Conexiones activas a DB
pg_stat_database_numbackends{datname="tickleveldb"}

# 6. Throughput por exchange
sum(rate(tickstream_producer_messages_sent_total{exchange=~".*"}[1m])) by (exchange)

# 7. Tamaño DB en GB
pg_database_size_bytes{datname="tickleveldb"} / 1024 / 1024 / 1024

# Uptime en horas
tickstream_producer_uptime_seconds / 3600

# Ver uptime por exchange
tickstream_producer_uptime_seconds{exchange="binance"}

# Ver session_id actual
tickstream_producer_session_info

```

---

## Alertas Recomendadas

| Alerta | Condición | Severidad |
|--------|-----------|-----------|
| Kafka LAG alto | `kafka_consumergroup_lag > 10000` | warning |
| Productores fallando | `rate(tickstream_producer_messages_failed_total[5m]) > 0` | critical |
| DB sin inserts | `rate(pg_stat_database_tup_inserted[5m]) == 0` | warning |
| Conexiones DB altas | `pg_stat_database_numbackends > 80` | warning |
| Cache hit bajo | `pg_stat_database_blks_hit / (blks_hit + blks_read) < 0.8` | warning |


# Health check
curl http://localhost:8080/health

# Trades
curl http://localhost:8080/api/v1/trades
curl "http://localhost:8080/api/v1/trades?symbol=BTC&limit=50"

# Anomalies
curl http://localhost:8080/api/v1/anomalies
curl http://localhost:8080/api/v1/anomalies/recent

# Metrics
curl http://localhost:8080/api/v1/metrics

# Prometheus metrics
curl http://localhost:8080/metrics
