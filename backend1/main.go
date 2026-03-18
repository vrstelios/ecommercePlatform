package main

import (
	"ecommercePlatform/backend1/api"
	"ecommercePlatform/config"
	"fmt"
	"github.com/gin-gonic/gin"
)

// Cassandra είναι εξαιρετική στο να διαχειρίζεται εκατομμύρια writes το δευτερόλεπτο
// H Cassandra υποστηρίζει το Lightweight Transaction (LWT). Αυτό αντικαθιστά το mu.Lock().

func main() {
	cfg, _ := config.LoadConfig("C:/Users/User/GolandProjects/ecommercePlatform/config/config.json")

	// Connect to Cassandra
	session := cfg.ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := cfg.ConnectRedis()
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
