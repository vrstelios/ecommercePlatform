package main

import (
	"context"
	"ecommercePlatform/backend2/api"
	pb "ecommercePlatform/backend2/proto"
	"ecommercePlatform/config"
	"ecommercePlatform/utils"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"log"
)

func main() {
	cfg, err := config.LoadConfig(config.FilePath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to Postgres
	pdb := cfg.ConnectPostgres()
	defer pdb.Close(context.Background())

	// Connect to ElasticSearch
	es, err := elasticsearch.NewDefaultClient()
	if err != nil {
		log.Fatalf("ES Client Error:  %s", err)
	}

	fmt.Println("DEBUG: Kafka Broker from config is:", cfg.Kafka.Broker)
	// Start the background worker to sync products to Elasticsearch
	go api.StartElasticSyncWorker(es, cfg.Kafka.Broker, "product-updates")

	// Connect to gRPC
	lis := cfg.ConnectGRPC()
	grpcServer := grpc.NewServer() // create gRPC server
	pb.RegisterProductServiceServer(grpcServer, &api.ProductServer{Es: es})

	go func() {
		log.Println("gRPC Server is running on port :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	router := gin.Default()
	logger, _ := zap.NewProduction()

	// Use elasticsearch for products
	router.GET("/products", func(c *gin.Context) {
		reqLogger := utils.GetLoggerWithTrace(c, logger)
		reqLogger.Info("Inventory get product")
		api.GetProduct(c, es)
	})
	router.POST("/products", func(c *gin.Context) {
		reqLogger := utils.GetLoggerWithTrace(c, logger)
		reqLogger.Info("Inventory create product")
		api.PostProductsElastic(c, es, pdb)
	})
	/*router.GET("/products/v2", func(c *gin.Context) {
		reqLogger := utils.GetLoggerWithTrace(c, logger)
		reqLogger.Info("Inventory get product")
		api.GetProductsElastic(c, es)
	})*/

	logger.Info(`backend-product running on port 8082`)

	router.Run(":8082")
}
