package models

import "time"

type Trade struct {
	ID        int64     `json:"id"`
	Ts        time.Time `json:"ts"`
	Symbol    string    `json:"symbol"`
	Exchange  string    `json:"exchange"`
	Price     float64   `json:"price"`
	Qty       float64   `json:"qty"`
	TradeID   string    `json:"trade_id"`
	Side      string    `json:"side"`
}

type Anomaly struct {
	ID            int64     `json:"id"`
	Ts            time.Time `json:"ts"`
	Symbol        string    `json:"symbol"`
	WindowStart   time.Time `json:"window_start"`
	WindowEnd     time.Time `json:"window_end"`
	MeanPrice     float64   `json:"mean_price"`
	StdPrice      float64   `json:"std_price"`
	LastPrice     float64   `json:"last_price"`
	ZScore        float64   `json:"z_score"`
	IsAnomaly     bool      `json:"is_anomaly"`
	AnomalyType   string    `json:"anomaly_type"`
	TradeCount    int64     `json:"trade_count"`
}

type RecentAnomaly struct {
	Symbol       string    `json:"symbol"`
	AnomalyCount int       `json:"anomaly_count"`
	MaxZScore    float64   `json:"max_z_score"`
	TotalWindows int       `json:"total_windows"`
	LastUpdate   time.Time `json:"last_update"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}
