package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/tickstream/api/models"
)

var allowedIntervals = map[string]bool{
	"1 second":  true,
	"5 seconds": true,
	"1 minute":  true,
	"5 minutes": true,
	"1 hour":    true,
}

type Handler struct {
	DB *sql.DB
}

func NewHandler(host string, port int, user, password, dbname string) (*Handler, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname,
	)

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

func parseLimit(raw string, defaultVal, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultVal
	}
	if n > max {
		return max
	}
	return n
}

func safeInterval(raw string) string {
	if allowedIntervals[raw] {
		return raw
	}
	return "1 minute"
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
	limit := parseLimit(c.DefaultQuery("limit", "100"), 100, 1000)

	var (
		rows *sql.Rows
		err  error
	)

	base := `SELECT id, ts, symbol, exchange, price, qty, trade_id, side FROM trades`

	if symbol != "" {
		rows, err = h.DB.Query(base+` WHERE symbol = $1 ORDER BY ts DESC LIMIT $2`, symbol, limit)
	} else {
		rows, err = h.DB.Query(base+` ORDER BY ts DESC LIMIT $1`, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	trades := make([]models.Trade, 0)
	for rows.Next() {
		var t models.Trade
		if err := rows.Scan(&t.ID, &t.Ts, &t.Symbol, &t.Exchange, &t.Price, &t.Qty, &t.TradeID, &t.Side); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		trades = append(trades, t)
	}

	c.JSON(http.StatusOK, trades)
}

func (h *Handler) GetAnomalies(c *gin.Context) {
	symbol := c.Query("symbol")
	limit := parseLimit(c.DefaultQuery("limit", "100"), 100, 1000)

	base := `
		SELECT id, ts, symbol, window_start, window_end, mean_price, std_price,
		       last_price, z_score, is_anomaly, anomaly_type, trade_count
		FROM anomalies`

	var (
		rows *sql.Rows
		err  error
	)

	if symbol != "" {
		rows, err = h.DB.Query(base+` WHERE symbol = $1 ORDER BY ts DESC LIMIT $2`, symbol, limit)
	} else {
		rows, err = h.DB.Query(base+` ORDER BY ts DESC LIMIT $1`, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	anomalies := make([]models.Anomaly, 0)
	for rows.Next() {
		var a models.Anomaly
		if err := rows.Scan(
			&a.ID, &a.Ts, &a.Symbol, &a.WindowStart, &a.WindowEnd,
			&a.MeanPrice, &a.StdPrice, &a.LastPrice, &a.ZScore,
			&a.IsAnomaly, &a.AnomalyType, &a.TradeCount,
		); err != nil {
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
		LIMIT 20`

	rows, err := h.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	anomalies := make([]models.RecentAnomaly, 0)
	for rows.Next() {
		var a models.RecentAnomaly
		if err := rows.Scan(&a.Symbol, &a.AnomalyCount, &a.MaxZScore, &a.TotalWindows, &a.LastUpdate); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		anomalies = append(anomalies, a)
	}

	c.JSON(http.StatusOK, anomalies)
}

func (h *Handler) GetMetrics(c *gin.Context) {
	symbol := c.Query("symbol")

	base := `
		SELECT
			time_bucket('1 minute', ts) AS bucket,
			AVG(price) AS avg_price,
			MIN(price) AS min_price,
			MAX(price) AS max_price,
			SUM(qty) AS total_volume,
			COUNT(*) AS trade_count
		FROM trades`

	var (
		rows *sql.Rows
		err  error
	)

	if symbol != "" {
		rows, err = h.DB.Query(base+` WHERE symbol = $1 GROUP BY bucket ORDER BY bucket DESC LIMIT 60`, symbol)
	} else {
		rows, err = h.DB.Query(base + ` GROUP BY bucket ORDER BY bucket DESC LIMIT 60`)
	}

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

	metrics := make([]Metric, 0)
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.Bucket, &m.AvgPrice, &m.MinPrice, &m.MaxPrice, &m.TotalVolume, &m.TradeCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		metrics = append(metrics, m)
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) GetAnomaliesChart(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := safeInterval(c.DefaultQuery("interval", "1 minute"))
	limit := parseLimit(c.DefaultQuery("limit", "60"), 60, 1440)

	base := fmt.Sprintf(`
		SELECT
			CAST(EXTRACT(EPOCH FROM time_bucket('%s', ts)) AS BIGINT) * 1000 AS bucket_time,
			COUNT(*) FILTER (WHERE is_anomaly) AS anomaly_count,
			COUNT(*) AS total_windows,
			MAX(z_score) AS max_z_score,
			AVG(z_score) AS avg_z_score
		FROM anomalies`, interval)

	var (
		rows *sql.Rows
		err  error
	)

	if symbol != "" {
		rows, err = h.DB.Query(base+` WHERE symbol = $1 GROUP BY 1 ORDER BY 1 DESC LIMIT $2`, symbol, limit)
	} else {
		rows, err = h.DB.Query(base+` GROUP BY 1 ORDER BY 1 DESC LIMIT $1`, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type ChartPoint struct {
		Time         int64   `json:"time"`
		AnomalyCount int64   `json:"anomaly_count"`
		TotalWindows int64   `json:"total_windows"`
		MaxZScore    float64 `json:"max_z_score"`
		AvgZScore    float64 `json:"avg_z_score"`
	}

	data := make([]ChartPoint, 0)
	for rows.Next() {
		var p ChartPoint
		var bucketTime int64
		if err := rows.Scan(&bucketTime, &p.AnomalyCount, &p.TotalWindows, &p.MaxZScore, &p.AvgZScore); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		p.Time = bucketTime
		data = append(data, p)
	}

	c.JSON(http.StatusOK, data)
}

func (h *Handler) GetTradesChart(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := safeInterval(c.DefaultQuery("interval", "1 minute"))
	limit := parseLimit(c.DefaultQuery("limit", "60"), 60, 1440)

	base := fmt.Sprintf(`
		SELECT
			CAST(EXTRACT(EPOCH FROM time_bucket('%s', ts)) AS BIGINT) * 1000 AS bucket_time,
			AVG(price) AS avg_price,
			MIN(price) AS min_price,
			MAX(price) AS max_price,
			SUM(qty) AS total_volume,
			COUNT(*) AS trade_count
		FROM trades`, interval)

	var (
		rows *sql.Rows
		err  error
	)

	if symbol != "" {
		rows, err = h.DB.Query(base+` WHERE symbol = $1 GROUP BY 1 ORDER BY 1 DESC LIMIT $2`, symbol, limit)
	} else {
		rows, err = h.DB.Query(base+` GROUP BY 1 ORDER BY 1 DESC LIMIT $1`, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type TradeChartPoint struct {
		Time        int64   `json:"time"`
		AvgPrice    float64 `json:"avg_price"`
		MinPrice    float64 `json:"min_price"`
		MaxPrice    float64 `json:"max_price"`
		TotalVolume float64 `json:"total_volume"`
		TradeCount  int64   `json:"trade_count"`
	}

	data := make([]TradeChartPoint, 0)
	for rows.Next() {
		var p TradeChartPoint
		var bucketTime int64
		if err := rows.Scan(&bucketTime, &p.AvgPrice, &p.MinPrice, &p.MaxPrice, &p.TotalVolume, &p.TradeCount); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		p.Time = bucketTime
		data = append(data, p)
	}

	c.JSON(http.StatusOK, data)
}

func (h *Handler) Close() {
	if h.DB != nil {
		h.DB.Close()
	}
}

// New endpoint: Get available symbols
func (h *Handler) GetSymbols(c *gin.Context) {
	query := `
		SELECT symbol, COUNT(*) as trade_count, MAX(ts) as last_update
		FROM trades
		GROUP BY symbol
		ORDER BY trade_count DESC
		LIMIT 50
	`

	rows, err := h.DB.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type SymbolInfo struct {
		Symbol     string  `json:"symbol"`
		TradeCount int64   `json:"trade_count"`
		LastUpdate *string `json:"last_update"`
	}

	data := make([]SymbolInfo, 0)
	for rows.Next() {
		var s SymbolInfo
		var lastUpdate sql.NullString
		if err := rows.Scan(&s.Symbol, &s.TradeCount, &lastUpdate); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if lastUpdate.Valid {
			s.LastUpdate = &lastUpdate.String
		}
		data = append(data, s)
	}

	c.JSON(http.StatusOK, data)
}
