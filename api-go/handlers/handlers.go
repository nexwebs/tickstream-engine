package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tickstream/api/models"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tickstream_api_requests_total",
			Help: "Total number of API requests",
		},
		[]string{"endpoint", "method"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tickstream_api_request_duration_seconds",
			Help:    "API request duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"endpoint"},
	)
	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "tickstream_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"query"},
	)
	dbConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tickstream_api_db_connections_active",
			Help: "Active database connections",
		},
	)
)

type Handler struct {
	DB *sql.DB
}

func NewHandler(host string, port int, user, password, dbname string) (*Handler, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &Handler{DB: db}, nil
}

func (h *Handler) Health(c *gin.Context) {
	start := time.Now()
	defer func() {
		requestsTotal.WithLabelValues("/health", c.Request.Method).Inc()
		requestDuration.WithLabelValues("/health").Observe(time.Since(start).Seconds())
	}()
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:  "ok",
		Service: "tickstream-api",
		Version: "1.0.0",
	})
}

func (h *Handler) GetTrades(c *gin.Context) {
	start := time.Now()
	symbol := c.Query("symbol")
	limit := c.DefaultQuery("limit", "100")

	query := `
		SELECT id, ts, symbol, exchange, price, qty, trade_id, side
		FROM trades
	`
	args := []interface{}{}
	conditions := ""

	if symbol != "" {
		conditions += " WHERE symbol = $1"
		args = append(args, symbol)
		if limit != "" {
			conditions += fmt.Sprintf(" ORDER BY ts DESC LIMIT $2")
			args = append(args, limit)
		}
	} else {
		conditions += fmt.Sprintf(" ORDER BY ts DESC LIMIT $1")
		args = append(args, limit)
	}

	query += conditions

	qStart := time.Now()
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_trades").Observe(time.Since(qStart).Seconds())
	dbConnectionsActive.Set(float64(h.DB.Stats().InUse))

	var trades []models.Trade
	for rows.Next() {
		var t models.Trade
		err := rows.Scan(&t.ID, &t.Ts, &t.Symbol, &t.Exchange, &t.Price, &t.Qty, &t.TradeID, &t.Side)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		trades = append(trades, t)
	}

	requestsTotal.WithLabelValues("/api/v1/trades", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/trades").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, trades)
}

func (h *Handler) GetAnomalies(c *gin.Context) {
	start := time.Now()
	symbol := c.Query("symbol")
	limit := c.DefaultQuery("limit", "100")

	query := `
		SELECT id, ts, symbol, window_start, window_end, mean_price, std_price,
		       last_price, z_score, is_anomaly, anomaly_type, trade_count
		FROM anomalies
	`
	args := []interface{}{}

	if symbol != "" {
		query += " WHERE symbol = $1 ORDER BY ts DESC LIMIT $2"
		args = append(args, symbol, limit)
	} else {
		query += " ORDER BY ts DESC LIMIT $1"
		args = append(args, limit)
	}

	qStart := time.Now()
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_anomalies").Observe(time.Since(qStart).Seconds())

	var anomalies []models.Anomaly
	for rows.Next() {
		var a models.Anomaly
		err := rows.Scan(&a.ID, &a.Ts, &a.Symbol, &a.WindowStart, &a.WindowEnd,
			&a.MeanPrice, &a.StdPrice, &a.LastPrice, &a.ZScore,
			&a.IsAnomaly, &a.AnomalyType, &a.TradeCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		anomalies = append(anomalies, a)
	}

	requestsTotal.WithLabelValues("/api/v1/anomalies", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/anomalies").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, anomalies)
}

func (h *Handler) GetRecentAnomalies(c *gin.Context) {
	start := time.Now()
	query := `
		SELECT 
			symbol,
			COUNT(*) FILTER (WHERE is_anomaly) AS anomaly_count,
			MAX(z_score) AS max_z_score,
			COUNT(*) AS total_windows,
			MAX(ts) AS last_update
		FROM anomalies
		WHERE ts > NOW() - INTERVAL '1 hour'
		GROUP BY symbol
		ORDER BY anomaly_count DESC
		LIMIT 20
	`

	qStart := time.Now()
	rows, err := h.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_recent_anomalies").Observe(time.Since(qStart).Seconds())

	var anomalies []models.RecentAnomaly
	for rows.Next() {
		var a models.RecentAnomaly
		err := rows.Scan(&a.Symbol, &a.AnomalyCount, &a.MaxZScore, &a.TotalWindows, &a.LastUpdate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		anomalies = append(anomalies, a)
	}

	requestsTotal.WithLabelValues("/api/v1/anomalies/recent", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/anomalies/recent").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, anomalies)
}

func (h *Handler) GetMetrics(c *gin.Context) {
	start := time.Now()
	symbol := c.Query("symbol")

	query := `
		SELECT 
			time_bucket('1 minute', ts) AS bucket,
			AVG(price) AS avg_price,
			MIN(price) AS min_price,
			MAX(price) AS max_price,
			SUM(qty) AS total_volume,
			COUNT(*) AS trade_count
		FROM trades
	`

	args := []interface{}{}
	if symbol != "" {
		query += " WHERE symbol = $1"
		args = append(args, symbol)
	}

	query += " GROUP BY bucket ORDER BY bucket DESC LIMIT 60"

	qStart := time.Now()
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_metrics").Observe(time.Since(qStart).Seconds())

	type Metric struct {
		Bucket      time.Time `json:"bucket"`
		AvgPrice    float64   `json:"avg_price"`
		MinPrice    float64   `json:"min_price"`
		MaxPrice    float64   `json:"max_price"`
		TotalVolume float64   `json:"total_volume"`
		TradeCount  int64     `json:"trade_count"`
	}

	var metrics []Metric
	for rows.Next() {
		var m Metric
		err := rows.Scan(&m.Bucket, &m.AvgPrice, &m.MinPrice, &m.MaxPrice, &m.TotalVolume, &m.TradeCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		metrics = append(metrics, m)
	}

	requestsTotal.WithLabelValues("/api/v1/metrics", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/metrics").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) Close() {
	if h.DB != nil {
		h.DB.Close()
	}
}

// New endpoint: Time-series for Grafana charts
func (h *Handler) GetAnomaliesChart(c *gin.Context) {
	start := time.Now()
	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", "1 minute")
	limit := c.DefaultQuery("limit", "60")

	// Grafana format: array of {time, value} objects
	query := `
		SELECT 
			EXTRACT(EPOCH FROM time_bucket('` + interval + `', ts))::bigint * 1000 AS time_ms,
			COUNT(*) FILTER (WHERE is_anomaly) AS anomaly_count,
			COUNT(*) AS total_windows,
			MAX(z_score) AS max_z_score,
			AVG(z_score) AS avg_z_score
		FROM anomalies
	`
	args := []interface{}{}

	if symbol != "" {
		query += " WHERE symbol = $1"
		args = append(args, symbol)
		query += " GROUP BY time_ms ORDER BY time_ms DESC LIMIT $2"
		args = append(args, limit)
	} else {
		query += " GROUP BY time_ms ORDER BY time_ms DESC LIMIT $1"
		args = append(args, limit)
	}

	qStart := time.Now()
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_anomalies_chart").Observe(time.Since(qStart).Seconds())

	type ChartPoint struct {
		Time         int64   `json:"time"`
		AnomalyCount int64   `json:"anomaly_count"`
		TotalWindows int64   `json:"total_windows"`
		MaxZScore    float64 `json:"max_z_score"`
		AvgZScore    float64 `json:"avg_z_score"`
	}

	var data []ChartPoint
	for rows.Next() {
		var p ChartPoint
		err := rows.Scan(&p.Time, &p.AnomalyCount, &p.TotalWindows, &p.MaxZScore, &p.AvgZScore)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		data = append(data, p)
	}

	requestsTotal.WithLabelValues("/api/v1/anomalies/chart", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/anomalies/chart").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, data)
}

// New endpoint: Trades time-series for Grafana
func (h *Handler) GetTradesChart(c *gin.Context) {
	start := time.Now()
	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", "1 minute")
	limit := c.DefaultQuery("limit", "60")

	// Grafana format: array of {time, value} objects
	query := `
		SELECT 
			EXTRACT(EPOCH FROM time_bucket('` + interval + `', ts))::bigint * 1000 AS time_ms,
			AVG(price) AS avg_price,
			MIN(price) AS min_price,
			MAX(price) AS max_price,
			SUM(qty) AS total_volume,
			COUNT(*) AS trade_count
		FROM trades
	`
	args := []interface{}{}

	if symbol != "" {
		query += " WHERE symbol = $1"
		args = append(args, symbol)
		query += " GROUP BY time_ms ORDER BY time_ms DESC LIMIT $2"
		args = append(args, limit)
	} else {
		query += " GROUP BY time_ms ORDER BY time_ms DESC LIMIT $1"
		args = append(args, limit)
	}

	qStart := time.Now()
	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	dbQueryDuration.WithLabelValues("get_trades_chart").Observe(time.Since(qStart).Seconds())

	type TradeChartPoint struct {
		Time        int64   `json:"time"`
		AvgPrice    float64 `json:"avg_price"`
		MinPrice    float64 `json:"min_price"`
		MaxPrice    float64 `json:"max_price"`
		TotalVolume float64 `json:"total_volume"`
		TradeCount  int64   `json:"trade_count"`
	}

	var data []TradeChartPoint
	for rows.Next() {
		var p TradeChartPoint
		err := rows.Scan(&p.Time, &p.AvgPrice, &p.MinPrice, &p.MaxPrice, &p.TotalVolume, &p.TradeCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		data = append(data, p)
	}

	requestsTotal.WithLabelValues("/api/v1/trades/chart", c.Request.Method).Inc()
	requestDuration.WithLabelValues("/api/v1/trades/chart").Observe(time.Since(start).Seconds())
	c.JSON(http.StatusOK, data)
}
