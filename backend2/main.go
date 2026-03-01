package main

import (
	"ecommercePlatform/backend2/api"
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
	router := gin.Default()

	es, err := elasticsearch.NewDefaultClient()
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	// Use elasticsearch for products
	router.GET("/products", func(c *gin.Context) {
		api.GetProduct(c, es)
	})
	router.POST("/products", func(c *gin.Context) {
		api.PostProductsElastic(c, es)
	})
	router.GET("/products/v2", func(c *gin.Context) {
		api.GetProductsElastic(c, es)
	})

	fmt.Println(`backend-product running on port 8085`)

	router.Run("localhost:8085")
}
