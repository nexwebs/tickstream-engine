package com.jobs.tickstream.flink;

import org.apache.flink.api.common.eventtime.WatermarkStrategy;
import org.apache.flink.api.common.functions.AggregateFunction;
import org.apache.flink.api.common.functions.MapFunction;
import org.apache.flink.api.common.serialization.SimpleStringSchema;
import org.apache.flink.api.connector.sink2.Sink;
import org.apache.flink.api.connector.sink2.SinkWriter;
import org.apache.flink.api.connector.sink2.WriterInitContext;
import org.apache.flink.connector.kafka.sink.KafkaRecordSerializationSchema;
import org.apache.flink.connector.kafka.sink.KafkaSink;
import org.apache.flink.connector.kafka.source.KafkaSource;
import org.apache.flink.connector.kafka.source.enumerator.initializer.OffsetsInitializer;
import org.apache.flink.streaming.api.datastream.DataStream;
import org.apache.flink.streaming.api.environment.StreamExecutionEnvironment;
import org.apache.flink.streaming.api.windowing.assigners.SlidingEventTimeWindows;

import com.google.gson.Gson;
import com.jobs.tickstream.flink.model.Trade;
import com.jobs.tickstream.flink.model.WindowAggregate;

import java.io.IOException;
import java.sql.*;
import java.time.Duration;

public class AnomalyDetectorJob {

    public static void main(String[] args) throws Exception {
        System.out.println(">>> AnomalyDetectorJob.main() started <<<");
        
        String kafkaBootstrapServers = "localhost:9092";
        String inputTopic = "trades-raw";
        String outputTopic = "trades-anomaly";
        int windowSizeSeconds = 30;
        int windowSlideSeconds = 10;
        
        String dbHost = "localhost";
        int dbPort = 5432;
        String dbName = "tickleveldb";
        String dbUser = "postgres";
        String dbPassword = "adminp";

        for (int i = 0; i < args.length; i++) {
            if (args[i].equals("--kafka.bootstrap.servers") && i + 1 < args.length) {
                kafkaBootstrapServers = args[i + 1];
            } else if (args[i].equals("--input.topic") && i + 1 < args.length) {
                inputTopic = args[i + 1];
            } else if (args[i].equals("--output.topic") && i + 1 < args.length) {
                outputTopic = args[i + 1];
            } else if (args[i].equals("--window.size.seconds") && i + 1 < args.length) {
                windowSizeSeconds = Integer.parseInt(args[i + 1]);
            } else if (args[i].equals("--window.slide.seconds") && i + 1 < args.length) {
                windowSlideSeconds = Integer.parseInt(args[i + 1]);
            } else if (args[i].equals("--db.host") && i + 1 < args.length) {
                dbHost = args[i + 1];
            } else if (args[i].equals("--db.port") && i + 1 < args.length) {
                dbPort = Integer.parseInt(args[i + 1]);
            } else if (args[i].equals("--db.name") && i + 1 < args.length) {
                dbName = args[i + 1];
            } else if (args[i].equals("--db.user") && i + 1 < args.length) {
                dbUser = args[i + 1];
            } else if (args[i].equals("--db.password") && i + 1 < args.length) {
                dbPassword = args[i + 1];
            }
        }

        String dbUrl = String.format("jdbc:postgresql://%s:%d/%s", dbHost, dbPort, dbName);

        StreamExecutionEnvironment env = StreamExecutionEnvironment.getExecutionEnvironment();
        env.getConfig().setClosureCleanerLevel(org.apache.flink.api.common.ExecutionConfig.ClosureCleanerLevel.NONE);
        env.enableCheckpointing(10000); // 10s checkpoint
        env.setParallelism(2); // Reduced for memory stability

        KafkaSource<String> kafkaSource = KafkaSource.<String>builder()
            .setBootstrapServers(kafkaBootstrapServers)
            .setTopics(inputTopic)
            .setGroupId("flink-anomaly-detector")
            .setStartingOffsets(OffsetsInitializer.earliest())
            .setValueOnlyDeserializer(new SimpleStringSchema())
            .build();

        DataStream<String> rawStream = env.fromSource(
            kafkaSource,
            WatermarkStrategy
                .<String>forBoundedOutOfOrderness(Duration.ofSeconds(5))
                .withTimestampAssigner((event, ts) -> {
                    try {
                        Gson gson = new Gson();
                        java.util.Map<String, Object> map = gson.fromJson(event, java.util.Map.class);
                        return ((Number) map.get("ts")).longValue();
                    } catch (Exception e) {
                        return System.currentTimeMillis();
                    }
                }),
            "Kafka Source"
        );

        DataStream<Trade> trades = rawStream
            .map(json -> {
                Gson gson = new Gson();
                try {
                    java.util.Map<String, Object> map = gson.fromJson(json, java.util.Map.class);
                    Trade trade = new Trade();
                    trade.setExchange(map.get("exchange").toString());
                    trade.setSymbol(map.get("symbol").toString());
                    trade.setPrice(((Number) map.get("price")).doubleValue());
                    trade.setQty(((Number) map.get("qty")).doubleValue());
                    trade.setTs(((Number) map.get("ts")).longValue());
                    trade.setTradeId(map.get("trade_id").toString());
                    trade.setSide(map.get("side").toString());
                    return trade;
                } catch (Exception e) {
                    return new Trade();
                }
            })
            .filter(trade -> trade.getSymbol() != null && !trade.getSymbol().isEmpty());

        // Persistir trades (datos de entrada)
        trades.sinkTo(new TradeDBSink(dbUrl, dbUser, dbPassword)).name("Trades to TimescaleDB");

        DataStream<WindowAggregate> windowed = trades
            .keyBy(Trade::getSymbol)
            .window(SlidingEventTimeWindows.of(
                Duration.ofSeconds(windowSizeSeconds),
                Duration.ofSeconds(windowSlideSeconds)))
            .aggregate(new TradeAggregateFunction(), new WindowMetadataFunction());

        // Persistir TODAS las ventanas (no solo anomalías)
        windowed.sinkTo(new TimescaleDBSink(dbUrl, dbUser, dbPassword)).name("All Windows to TimescaleDB");

        // Guardar TODAS las ventanas como anomalías (para debug)
        windowed.print().name("DEBUG: All Windows");
        
        // También filtrar anomalías para Kafka output y persistirlas
        DataStream<WindowAggregate> anomalies = windowed; // Guardar todas

        DataStream<String> anomalyJson = anomalies.map(a -> String.format(
            "{\"symbol\":\"%s\",\"z_score\":%.2f,\"is_anomaly\":%b,\"anomaly_type\":\"%s\"}",
            a.getSymbol(), a.getZScore(), a.isAnomaly(), a.getAnomalyType()
        ));

        KafkaSink<String> kafkaSink = KafkaSink.<String>builder()
            .setBootstrapServers(kafkaBootstrapServers)
            .setRecordSerializer(KafkaRecordSerializationSchema.builder()
                .setTopic(outputTopic)
                .setValueSerializationSchema(new SimpleStringSchema())
                .build())
            .build();

        // COMMENTED OUT para aislar el problema de DB
        // anomalyJson.sinkTo(kafkaSink).name("Anomalies to Kafka");

        anomalies.sinkTo(new TimescaleDBSink(dbUrl, dbUser, dbPassword)).name("Anomalies to TimescaleDB");

        anomalies.print().name("Print Anomalies");

        System.out.println(">>> Calling env.execute() - Flink job should start now <<<");
        env.execute("Anomaly Detection Job");
        System.out.println(">>> env.execute() returned - Flink job finished <<<");
    }

    public static class TradeParser implements MapFunction<String, Trade> {
        private final Gson gson = new Gson();

        @Override
        public Trade map(String json) {
            try {
                java.util.Map<String, Object> map = gson.fromJson(json, java.util.Map.class);
                Trade trade = new Trade();
                trade.setExchange(map.get("exchange").toString());
                trade.setSymbol(map.get("symbol").toString());
                trade.setPrice(((Number) map.get("price")).doubleValue());
                trade.setQty(((Number) map.get("qty")).doubleValue());
                trade.setTs(((Number) map.get("ts")).longValue());
                trade.setTradeId(map.get("trade_id").toString());
                trade.setSide(map.get("side").toString());
                return trade;
            } catch (Exception e) {
                return new Trade();
            }
        }
    }

    public static class WindowMetadataFunction extends org.apache.flink.streaming.api.functions.windowing.ProcessWindowFunction<WindowAggregate, WindowAggregate, String, org.apache.flink.streaming.api.windowing.windows.TimeWindow> {
        @Override
        public void process(String key, Context ctx, Iterable<WindowAggregate> elements, org.apache.flink.util.Collector<WindowAggregate> out) {
            WindowAggregate agg = elements.iterator().next();
            agg.setSymbol(key);
            agg.setWindowStart(ctx.window().getStart());
            agg.setWindowEnd(ctx.window().getEnd());
            out.collect(agg);
        }
    }

    public static class TradeAggregateFunction implements AggregateFunction<Trade, TradeAccumulator, WindowAggregate> {

        @Override
        public TradeAccumulator createAccumulator() {
            return new TradeAccumulator();
        }

        @Override
        public TradeAccumulator add(Trade value, TradeAccumulator accumulator) {
            accumulator.getPrices().add(value.getPrice());
            accumulator.setCount(accumulator.getCount() + 1);
            accumulator.setLastPrice(value.getPrice());
            return accumulator;
        }

        @Override
        public WindowAggregate getResult(TradeAccumulator accumulator) {
            if (accumulator.getPrices().isEmpty()) {
                return new WindowAggregate("UNKNOWN", 0, 0, 0.0, 0.0, 0, 0.0, 0.0, false, "NORMAL");
            }

            double mean = accumulator.getPrices().stream().mapToDouble(d -> d).average().orElse(0.0);
            double variance = accumulator.getPrices().stream()
                .mapToDouble(p -> Math.pow(p - mean, 2))
                .average().orElse(0.0);
            double std = Math.sqrt(variance);
            double zScore = std > 0 ? (accumulator.getLastPrice() - mean) / std : 0.0;
            // Detectar anomalía: cualquier variación significativa
            boolean isAnomaly = Math.abs(zScore) > 0.1 || accumulator.getCount() > 10;
            
            String anomalyType;
            if (zScore > 1.0) anomalyType = "SPIKE";
            else if (zScore < -1.0) anomalyType = "DROP";
            else if (accumulator.getCount() > 30) anomalyType = "VOLUME_SURGE";
            else anomalyType = "NORMAL";

            return new WindowAggregate("UNKNOWN", 0, 0, mean, std, 
                accumulator.getCount(), accumulator.getLastPrice(), zScore, isAnomaly, anomalyType);
        }

        @Override
        public TradeAccumulator merge(TradeAccumulator a, TradeAccumulator b) {
            a.getPrices().addAll(b.getPrices());
            a.setCount(a.getCount() + b.getCount());
            if (a.getCount() >= b.getCount()) {
                a.setLastPrice(a.getLastPrice());
            } else {
                a.setLastPrice(b.getLastPrice());
            }
            return a;
        }
    }

    public static class TradeAccumulator {
        private java.util.List<Double> prices = new java.util.ArrayList<>();
        private long count = 0;
        private double lastPrice = 0.0;

        public java.util.List<Double> getPrices() { return prices; }
        public void setPrices(java.util.List<Double> prices) { this.prices = prices; }
        public long getCount() { return count; }
        public void setCount(long count) { this.count = count; }
        public double getLastPrice() { return lastPrice; }
        public void setLastPrice(double lastPrice) { this.lastPrice = lastPrice; }
    }

    public static class TimescaleDBSink implements Sink<WindowAggregate> {
        private final String dbUrl;
        private final String dbUser;
        private final String dbPassword;

        public TimescaleDBSink(String dbUrl, String dbUser, String dbPassword) {
            this.dbUrl = dbUrl;
            this.dbUser = dbUser;
            this.dbPassword = dbPassword;
        }

        @Override
        public SinkWriter<WindowAggregate> createWriter(WriterInitContext context) throws IOException {
            try {
                return new TimescaleDBWriter(dbUrl, dbUser, dbPassword);
            } catch (SQLException e) {
                throw new IOException("Failed to initialize TimescaleDB writer", e);
            }
        }
    }

    public static class TimescaleDBWriter implements SinkWriter<WindowAggregate> {
        private transient Connection conn;
        private transient PreparedStatement pstmt;
        private int batchCount = 0;
        private static final int BATCH_SIZE = 50;

        public TimescaleDBWriter(String dbUrl, String dbUser, String dbPassword) throws SQLException {
            try {
                Class.forName("org.postgresql.Driver");
                conn = DriverManager.getConnection(dbUrl, dbUser, dbPassword);
                conn.setAutoCommit(false);
                pstmt = conn.prepareStatement(
                    "INSERT INTO anomalies (ts, symbol, window_start, window_end, mean_price, std_price, " +
                    "last_price, z_score, is_anomaly, anomaly_type, trade_count) " +
                    "VALUES (NOW(), ?, to_timestamp(?/1000.0), to_timestamp(?/1000.0), ?, ?, ?, ?, ?, ?, ?)");
            } catch (ClassNotFoundException e) {
                throw new SQLException("PostgreSQL driver not found", e);
            }
        }

        @Override
        public void write(WindowAggregate anomaly, SinkWriter.Context context) throws IOException {
            try {
                pstmt.setString(1, anomaly.getSymbol());
                pstmt.setLong(2, anomaly.getWindowStart());
                pstmt.setLong(3, anomaly.getWindowEnd());
                pstmt.setDouble(4, anomaly.getMeanPrice());
                pstmt.setDouble(5, anomaly.getStdPrice());
                pstmt.setDouble(6, anomaly.getLastPrice());
                pstmt.setDouble(7, anomaly.getZScore());
                pstmt.setBoolean(8, anomaly.isAnomaly());
                pstmt.setString(9, anomaly.getAnomalyType());
                pstmt.setLong(10, anomaly.getTradeCount());
                pstmt.addBatch();
                batchCount++;
                if (batchCount >= BATCH_SIZE) {
                    pstmt.executeBatch();
                    conn.commit();
                    batchCount = 0;
                }
            } catch (SQLException e) {
                throw new IOException("Failed to write to TimescaleDB", e);
            }
        }

        @Override
        public void flush(boolean endOfInput) throws IOException {
            try {
                if (batchCount > 0) {
                    pstmt.executeBatch();
                    conn.commit();
                    batchCount = 0;
                }
            } catch (SQLException e) {
                throw new IOException("Failed to flush to TimescaleDB", e);
            }
        }

        @Override
        public void close() throws IOException {
            try {
                if (pstmt != null) {
                    pstmt.close();
                }
                if (conn != null) {
                    conn.close();
                }
            } catch (SQLException e) {
                throw new IOException("Failed to close TimescaleDB connection", e);
            }
        }
    }

    public static class TradeDBSink implements Sink<Trade> {
        private final String dbUrl;
        private final String dbUser;
        private final String dbPassword;

        public TradeDBSink(String dbUrl, String dbUser, String dbPassword) {
            this.dbUrl = dbUrl;
            this.dbUser = dbUser;
            this.dbPassword = dbPassword;
        }

        @Override
        public SinkWriter<Trade> createWriter(WriterInitContext context) throws IOException {
            try {
                return new TradeDBWriter(dbUrl, dbUser, dbPassword);
            } catch (SQLException e) {
                throw new IOException("Failed to initialize Trade DB writer", e);
            }
        }
    }

    public static class TradeDBWriter implements SinkWriter<Trade> {
        private transient Connection conn;
        private transient PreparedStatement pstmt;
        private int batchCount = 0;
        private static final int BATCH_SIZE = 100; // Smaller batches for stability

        public TradeDBWriter(String dbUrl, String dbUser, String dbPassword) throws SQLException {
            try {
                Class.forName("org.postgresql.Driver");
                conn = DriverManager.getConnection(dbUrl, dbUser, dbPassword);
                conn.setAutoCommit(false);
                conn.setNetworkTimeout(null, 10000);
                pstmt = conn.prepareStatement(
                    "INSERT INTO trades (ts, symbol, exchange, price, qty, trade_id, side) " +
                    "VALUES (to_timestamp(?/1000), ?, ?, ?, ?, ?, ?)");
            } catch (ClassNotFoundException e) {
                throw new SQLException("PostgreSQL driver not found", e);
            }
        }

        @Override
        public void write(Trade trade, SinkWriter.Context context) throws IOException {
            try {
                pstmt.setLong(1, trade.getTs());
                pstmt.setString(2, trade.getSymbol());
                pstmt.setString(3, trade.getExchange());
                pstmt.setDouble(4, trade.getPrice());
                pstmt.setDouble(5, trade.getQty());
                pstmt.setString(6, trade.getTradeId());
                pstmt.setString(7, trade.getSide());
                pstmt.addBatch();
                batchCount++;
                if (batchCount >= BATCH_SIZE) {
                    pstmt.executeBatch();
                    conn.commit();
                    batchCount = 0;
                }
            } catch (SQLException e) {
                throw new IOException("Failed to write trade to DB", e);
            }
        }

        @Override
        public void flush(boolean endOfInput) throws IOException {
            try {
                if (batchCount > 0) {
                    pstmt.executeBatch();
                    conn.commit();
                    batchCount = 0;
                }
            } catch (SQLException e) {
                throw new IOException("Failed to flush trades to DB", e);
            }
        }

        @Override
        public void close() throws IOException {
            try {
                if (pstmt != null) pstmt.close();
                if (conn != null) conn.close();
            } catch (SQLException e) {
                throw new IOException("Failed to close trade DB connection", e);
            }
        }
    }
}
