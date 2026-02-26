package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tickstream/api/handlers"
)

func main() {
	host := flag.String("host", "localhost", "Database host")
	port := flag.Int("port", 5432, "Database port")
	user := flag.String("user", "postgres", "Database user")
	password := flag.String("password", "adminp", "Database password")
	dbname := flag.String("db", "tickleveldb", "Database name")
	serverPort := flag.Int("server.port", 8080, "Server port")
	flag.Parse()

	handler, err := handlers.NewHandler(*host, *port, *user, *password, *dbname)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer handler.Close()

	log.Printf("Connected to TimescaleDB at %s:%d", *host, *port)

	r := gin.Default()

	r.GET("/health", handler.Health)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/api/v1")
	{
		api.GET("/docs", handler.GetDocs)
		api.GET("/symbols", handler.GetSymbols)
		api.GET("/trades", handler.GetTrades)
		api.GET("/trades/chart", handler.GetTradesChart)
		api.GET("/anomalies", handler.GetAnomalies)
		api.GET("/anomalies/chart", handler.GetAnomaliesChart)
		api.GET("/anomalies/recent", handler.GetRecentAnomalies)
		api.GET("/metrics", handler.GetMetrics)
	}

	go func() {
		addr := fmt.Sprintf(":%d", *serverPort)
		log.Printf("Starting server on %s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")
}
