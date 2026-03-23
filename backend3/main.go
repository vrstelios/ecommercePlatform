package main

import (
	"context"
	"ecommercePlatform/backend3/api"
	"ecommercePlatform/config"
	"ecommercePlatform/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
)

func main() {
	cfg, err := config.LoadConfig(config.FilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to Cassandra
	session := cfg.ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := cfg.ConnectRedis()
	defer rdb.Close()

	// Connect to Postgres
	pdb := cfg.ConnectPostgres()
	defer pdb.Close(context.Background())

	// Start Kafka Worker in Background
	go api.StartPaymentWorker(session, rdb, pdb)

	// Get Kafka Writer for API
	kafkaWriter := cfg.GetKafkaWriter()

	router := gin.Default()
	logger, _ := zap.NewProduction()

	router.POST("/orders/:id/pay", func(ctx *gin.Context) {
		reqLogger := utils.GetLoggerWithTrace(ctx, logger)
		reqLogger.Info("Payment request received")

		api.PostOrderPayment(ctx, rdb, kafkaWriter, reqLogger)
	})

	logger.Info(`backend-order running on port 8083`)

	router.Run("localhost:8083")
}
