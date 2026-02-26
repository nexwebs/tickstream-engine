package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GetDocs(c *gin.Context) {
	spec := gin.H{
		"openapi": "3.0.3",
		"info": gin.H{
			"title":       "Tickstream API",
			"description": "REST API for trade and anomaly time-series data",
			"version":     "1.0.0",
		},
		"servers": []gin.H{
			{"url": "/api/v1", "description": "Current environment"},
		},
		"paths": gin.H{
			"/health": gin.H{
				"get": gin.H{
					"summary":     "Health check",
					"operationId": "health",
					"tags":        []string{"system"},
					"responses": gin.H{
						"200": gin.H{
							"description": "Service is healthy",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{"$ref": "#/components/schemas/HealthResponse"},
									"example": gin.H{
										"status":  "ok",
										"service": "tickstream-api",
										"version": "1.0.0",
									},
								},
							},
						},
					},
				},
			},
			"/symbols": gin.H{
				"get": gin.H{
					"summary":     "List available trading symbols",
					"operationId": "getSymbols",
					"tags":        []string{"trades"},
					"description": "Returns list of symbols with trade count. Use these symbols in other endpoints.",
					"responses": gin.H{
						"200": gin.H{
							"description": "Array of available symbols",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type": "array",
										"items": gin.H{
											"type": "object",
											"properties": gin.H{
												"symbol":      gin.H{"type": "string", "example": "BTCUSD"},
												"trade_count": gin.H{"type": "integer"},
												"last_update": gin.H{"type": "string", "format": "date-time"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"/trades": gin.H{
				"get": gin.H{
					"summary":     "List recent trades",
					"operationId": "getTrades",
					"tags":        []string{"trades"},
					"parameters": []gin.H{
						{
							"name": "symbol", "in": "query", "required": false,
							"schema":      gin.H{"type": "string", "example": "BTCUSDT"},
							"description": "Filter by trading pair symbol",
						},
						{
							"name": "limit", "in": "query", "required": false,
							"schema":      gin.H{"type": "integer", "default": 100, "minimum": 1, "maximum": 1000},
							"description": "Number of records to return",
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Array of trades ordered by timestamp DESC",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/Trade"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
			"/trades/chart": gin.H{
				"get": gin.H{
					"summary":     "Trades time-series aggregated by interval",
					"operationId": "getTradesChart",
					"tags":        []string{"trades"},
					"parameters": []gin.H{
						{
							"name": "symbol", "in": "query", "required": false,
							"schema": gin.H{"type": "string"},
						},
						{
							"name": "interval", "in": "query", "required": false,
							"schema": gin.H{
								"type":    "string",
								"default": "1 minute",
								"enum":    []string{"1 second", "5 seconds", "1 minute", "5 minutes", "1 hour"},
							},
						},
						{
							"name": "limit", "in": "query", "required": false,
							"schema": gin.H{"type": "integer", "default": 60, "minimum": 1, "maximum": 1440},
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Aggregated OHLCV buckets",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/TradeChartPoint"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
			"/anomalies": gin.H{
				"get": gin.H{
					"summary":     "List anomaly detection windows",
					"operationId": "getAnomalies",
					"tags":        []string{"anomalies"},
					"parameters": []gin.H{
						{
							"name": "symbol", "in": "query", "required": false,
							"schema": gin.H{"type": "string"},
						},
						{
							"name": "limit", "in": "query", "required": false,
							"schema": gin.H{"type": "integer", "default": 100, "minimum": 1, "maximum": 1000},
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Array of anomaly windows ordered by timestamp DESC",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/Anomaly"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
			"/anomalies/recent": gin.H{
				"get": gin.H{
					"summary":     "Anomaly summary per symbol in the last hour",
					"operationId": "getRecentAnomalies",
					"tags":        []string{"anomalies"},
					"responses": gin.H{
						"200": gin.H{
							"description": "Top 20 symbols by anomaly count in the past hour",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/RecentAnomaly"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
			"/anomalies/chart": gin.H{
				"get": gin.H{
					"summary":     "Anomaly time-series aggregated by interval",
					"operationId": "getAnomaliesChart",
					"tags":        []string{"anomalies"},
					"parameters": []gin.H{
						{
							"name": "symbol", "in": "query", "required": false,
							"schema": gin.H{"type": "string"},
						},
						{
							"name": "interval", "in": "query", "required": false,
							"schema": gin.H{
								"type":    "string",
								"default": "1 minute",
								"enum":    []string{"1 second", "5 seconds", "1 minute", "5 minutes", "1 hour"},
							},
						},
						{
							"name": "limit", "in": "query", "required": false,
							"schema": gin.H{"type": "integer", "default": 60, "minimum": 1, "maximum": 1440},
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Anomaly count and z-score buckets",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/AnomalyChartPoint"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
			"/metrics": gin.H{
				"get": gin.H{
					"summary":     "1-minute OHLCV metrics (last 60 buckets)",
					"operationId": "getMetrics",
					"tags":        []string{"metrics"},
					"parameters": []gin.H{
						{
							"name": "symbol", "in": "query", "required": false,
							"schema": gin.H{"type": "string"},
						},
					},
					"responses": gin.H{
						"200": gin.H{
							"description": "Array of 1-minute metric buckets",
							"content": gin.H{
								"application/json": gin.H{
									"schema": gin.H{
										"type":  "array",
										"items": gin.H{"$ref": "#/components/schemas/Metric"},
									},
								},
							},
						},
						"500": gin.H{"$ref": "#/components/responses/InternalError"},
					},
				},
			},
		},
		"components": gin.H{
			"responses": gin.H{
				"InternalError": gin.H{
					"description": "Internal server error",
					"content": gin.H{
						"application/json": gin.H{
							"schema": gin.H{
								"type": "object",
								"properties": gin.H{
									"error": gin.H{"type": "string"},
								},
							},
						},
					},
				},
			},
			"schemas": gin.H{
				"HealthResponse": gin.H{
					"type": "object",
					"properties": gin.H{
						"status":  gin.H{"type": "string", "example": "ok"},
						"service": gin.H{"type": "string", "example": "tickstream-api"},
						"version": gin.H{"type": "string", "example": "1.0.0"},
					},
				},
				"Trade": gin.H{
					"type": "object",
					"properties": gin.H{
						"id":       gin.H{"type": "integer"},
						"ts":       gin.H{"type": "string", "format": "date-time"},
						"symbol":   gin.H{"type": "string"},
						"exchange": gin.H{"type": "string"},
						"price":    gin.H{"type": "number"},
						"qty":      gin.H{"type": "number"},
						"trade_id": gin.H{"type": "string"},
						"side":     gin.H{"type": "string", "enum": []string{"buy", "sell"}},
					},
				},
				"Anomaly": gin.H{
					"type": "object",
					"properties": gin.H{
						"id":           gin.H{"type": "integer"},
						"ts":           gin.H{"type": "string", "format": "date-time"},
						"symbol":       gin.H{"type": "string"},
						"window_start": gin.H{"type": "string", "format": "date-time"},
						"window_end":   gin.H{"type": "string", "format": "date-time"},
						"mean_price":   gin.H{"type": "number"},
						"std_price":    gin.H{"type": "number"},
						"last_price":   gin.H{"type": "number"},
						"z_score":      gin.H{"type": "number"},
						"is_anomaly":   gin.H{"type": "boolean"},
						"anomaly_type": gin.H{"type": "string"},
						"trade_count":  gin.H{"type": "integer"},
					},
				},
				"RecentAnomaly": gin.H{
					"type": "object",
					"properties": gin.H{
						"symbol":        gin.H{"type": "string"},
						"anomaly_count": gin.H{"type": "integer"},
						"max_z_score":   gin.H{"type": "number"},
						"total_windows": gin.H{"type": "integer"},
						"last_update":   gin.H{"type": "string", "format": "date-time"},
					},
				},
				"Metric": gin.H{
					"type": "object",
					"properties": gin.H{
						"bucket":       gin.H{"type": "string", "format": "date-time"},
						"avg_price":    gin.H{"type": "number"},
						"min_price":    gin.H{"type": "number"},
						"max_price":    gin.H{"type": "number"},
						"total_volume": gin.H{"type": "number"},
						"trade_count":  gin.H{"type": "integer"},
					},
				},
				"TradeChartPoint": gin.H{
					"type": "object",
					"properties": gin.H{
						"time":         gin.H{"type": "integer", "description": "Unix timestamp in milliseconds"},
						"avg_price":    gin.H{"type": "number"},
						"min_price":    gin.H{"type": "number"},
						"max_price":    gin.H{"type": "number"},
						"total_volume": gin.H{"type": "number"},
						"trade_count":  gin.H{"type": "integer"},
					},
				},
				"AnomalyChartPoint": gin.H{
					"type": "object",
					"properties": gin.H{
						"time":          gin.H{"type": "integer", "description": "Unix timestamp in milliseconds"},
						"anomaly_count": gin.H{"type": "integer"},
						"total_windows": gin.H{"type": "integer"},
						"max_z_score":   gin.H{"type": "number"},
						"avg_z_score":   gin.H{"type": "number"},
					},
				},
			},
		},
	}

	c.JSON(http.StatusOK, spec)
}
