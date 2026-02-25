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
	c.JSON(http.StatusOK, models.HealthResponse{
		Status:  "ok",
		Service: "tickstream-api",
		Version: "1.0.0",
	})
}

func (h *Handler) GetTrades(c *gin.Context) {
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

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

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

	c.JSON(http.StatusOK, trades)
}

func (h *Handler) GetAnomalies(c *gin.Context) {
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

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

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

	c.JSON(http.StatusOK, anomalies)
}

func (h *Handler) GetRecentAnomalies(c *gin.Context) {
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

	rows, err := h.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

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

	c.JSON(http.StatusOK, anomalies)
}

func (h *Handler) GetMetrics(c *gin.Context) {
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

	rows, err := h.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

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

	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) Close() {
	if h.DB != nil {
		h.DB.Close()
	}
}
