package main

import (
	"ecommercePlatform/backend1/api"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"log"
)

// Cassandra είναι εξαιρετική στο να διαχειρίζεται εκατομμύρια writes το δευτερόλεπτο
// H Cassandra υποστηρίζει το Lightweight Transaction (LWT). Αυτό αντικαθιστά το mu.Lock().

func main() {
	// Connect to Cassandra
	session := ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := ConnectRedis()
	defer rdb.Close()

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

		routerEndpoints.POST("/orders/create/:id", func(ctx *gin.Context) {
			api.CreateOrderFromCart(ctx, session, rdb)
		})
	}

	fmt.Println(`backend-cart&inventory running on port 8081`)

	router.Run("localhost:8081")
}

func ConnectRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	return rdb
}

func ConnectCassandra() *gocql.Session {
	cluster := gocql.NewCluster("localhost")
	cluster.Keyspace = "ecommerce"
	cluster.Consistency = gocql.Quorum
	cluster.Port = 9042

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Cassandra connection failed: %v", err)
	}

	return session
}
