package main

import (
	"ecommercePlatform/backend1/api"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"log"
)

// Cassandra είναι εξαιρετική στο να διαχειρίζεται εκατομμύρια writes το δευτερόλεπτο
// H Cassandra υποστηρίζει το Lightweight Transaction (LWT). Αυτό αντικαθιστά το mu.Lock() που είχες.
// Redis είναι εξαιρετική στο να διαχειρίζεται γρήγορα reads και writes με χαμηλή καθυστέρηση

func main() {
	cluster := gocql.NewCluster("localhost")
	cluster.Keyspace = "ecommerce"
	cluster.Consistency = gocql.Quorum
	cluster.Port = 9042

	// Create a session to interact with the Cassandra
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Cassandra connection failed: %v", err)
	}
	defer session.Close()

	router := gin.Default()

	routerEndpoints := router.Group("/api")
	{
		routerEndpoints.POST("/cart/items", func(c *gin.Context) {
			api.PostCartItems(c, session)
		})

		routerEndpoints.GET("/cart/items/:id", func(c *gin.Context) {
			api.GetCartItems(c, session)
		})

		routerEndpoints.POST("/inventory", func(c *gin.Context) {
			api.PostInventory(c, session)
		})

		routerEndpoints.GET("/inventory/:id", func(c *gin.Context) {
			api.GetInventory(c, session)
		})
	}

	fmt.Println(`backend-cart&inventory running on port 8081`)

	router.Run("localhost:8081")
}
