package main

import (
	"context"
	"ecommercePlatform/backend1/api"
	"ecommercePlatform/config"
	"ecommercePlatform/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
)

// Cassandra είναι εξαιρετική στο να διαχειρίζεται εκατομμύρια writes το δευτερόλεπτο
// H Cassandra υποστηρίζει το Lightweight Transaction (LWT). Αυτό αντικαθιστά το mu.Lock().

func main() {
	cfg, err := config.LoadConfig(config.FilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to Postgres
	pdb := cfg.ConnectPostgres()
	defer pdb.Close(context.Background())

	// Connect to Cassandra
	session := cfg.ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := cfg.ConnectRedis()
	defer rdb.Close()

	// Get Kafka Writer for API
	kafkaWriter := cfg.GetProductsKafkaWriter()

	router := gin.Default()
	logger, _ := zap.NewProduction()

	routerEndpoints := router.Group("/api")
	{
		routerEndpoints.POST("/cart/items", func(c *gin.Context) {
			reqLogger := utils.GetLoggerWithTrace(c, logger)
			reqLogger.Info("Inventory create item")
			api.PostCartItems(c, session)
		})

		routerEndpoints.GET("/cart/items/:id", func(c *gin.Context) {
			reqLogger := utils.GetLoggerWithTrace(c, logger)
			reqLogger.Info("Inventory get item")
			api.GetCartItems(c, session)
		})

		routerEndpoints.POST("/inventory", func(c *gin.Context) {
			reqLogger := utils.GetLoggerWithTrace(c, logger)
			reqLogger.Info("Inventory create received")
			api.PostInventory(c, session, kafkaWriter, pdb)
		})

		routerEndpoints.GET("/inventory/:id", func(c *gin.Context) {
			reqLogger := utils.GetLoggerWithTrace(c, logger)
			reqLogger.Info("Inventory get received")
			api.GetInventory(c, session)
		})

		routerEndpoints.POST("/orders/create/:id", func(ctx *gin.Context) {
			reqLogger := utils.GetLoggerWithTrace(ctx, logger)
			reqLogger.Info("Inventory create order")
			api.CreateOrderFromCart(ctx, session, rdb)
		})
	}

	logger.Info(`backend-cart&inventory running on port 8081`)

	router.Run("localhost:8081")
}
