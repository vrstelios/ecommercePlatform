package main

import (
	"context"
	"ecommercePlatform/backend2/api"
	"ecommercePlatform/config"
	"ecommercePlatform/utils"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
)

/*Το Elasticsearch δεν είναι απλά μια βάση, είναι μια μηχανή
αναζήτησης που επιτρέπει "fuzzy search" (π.χ. να γράφεις
"phne" και να βρίσκει "phone") και είναι ταχύτατο σε τεράστιο
όγκο δεδομένων.*/

// run -> ttp://localhost:9200 in browser to see if elasticsearch is running

func main() {
	cfg, err := config.LoadConfig(config.FilePath)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to Postgres
	pdb := cfg.ConnectPostgres()
	defer pdb.Close(context.Background())

	// Connect to ElasticSearch
	es, err := elasticsearch.NewDefaultClient()
	if err != nil {
		log.Fatalf("ES Client Error:  %s", err)
	}

	// Start the background worker to sync products to Elasticsearch
	go api.StartElasticSyncWorker(es, cfg.Kafka.Broker, "product-updates")

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
	router.GET("/products/v2", func(c *gin.Context) {
		reqLogger := utils.GetLoggerWithTrace(c, logger)
		reqLogger.Info("Inventory get product")
		api.GetProductsElastic(c, es)
	})

	logger.Info(`backend-product running on port 8082`)

	router.Run("localhost:8082")
}
