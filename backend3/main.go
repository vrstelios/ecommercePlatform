package main

import (
	"context"
	"ecommercePlatform/backend3/api"
	"ecommercePlatform/config"
	"ecommercePlatform/utils"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
)

func main() {
	cfg, err := config.LoadConfig(config.FilePath)
	if err != nil {
		cfg, err = config.LoadConfig("config/config-localHost.json")
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
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

	fmt.Println("DEBUG: Kafka Broker from config is:", cfg.Kafka.Broker)
	// Start Kafka Worker in Background
	go api.StartPaymentWorker(session, rdb, pdb, cfg.Kafka.Broker)

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

	//router.Run("localhost:8083")
	router.Run(":8083")
}
