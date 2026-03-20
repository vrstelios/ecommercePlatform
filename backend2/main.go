package main

import (
	"context"
	"ecommercePlatform/backend2/api"
	"ecommercePlatform/config"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
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

	// Use elasticsearch for products
	router.GET("/products", func(c *gin.Context) {
		api.GetProduct(c, es)
	})
	router.POST("/products", func(c *gin.Context) {
		api.PostProductsElastic(c, es, pdb)
	})
	router.GET("/products/v2", func(c *gin.Context) {
		api.GetProductsElastic(c, es)
	})

	fmt.Println(`backend-product running on port 8082`)

	router.Run("localhost:8082")
}
