package main

import (
	"ecommercePlatform/backend3/api"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"log"
)

//Είναι σχεδιασμένος για δεδομένα «σε κίνηση». Διαχειρίζεται συνεχείς ροές πληροφοριών
//(streams) σε πραγματικό χρόνο. Δεν περιμένει να τον ρωτήσεις· σου «σπρώχνει» την
//πληροφορία τη στιγμή που συμβαίνει.

/*Ασύγχρονη Επικοινωνία με Kafka (Event-Driven)
Όταν ο χρήστης πατάει "Checkout", το Cart Service στέλνει ένα μήνυμα στον
Kafka: "Ο χρήστης X θέλει να αγοράσει αυτά τα 5 προϊόντα". Το Order Service
"ακούει" αυτό το μήνυμα και δημιουργεί την παραγγελία στη δική του βάση.*/

func main() {
	// Connect to Cassandra
	session := ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := ConnectRedis()
	defer rdb.Close()

	// Start Kafka Worker in Background
	go api.StartPaymentWorker(session, rdb)

	kafkaWriter := &kafka.Writer{
		Addr:     kafka.TCP("localhost:9092"),
		Topic:    "payment-events",
		Balancer: &kafka.LeastBytes{},
	}

	router := gin.Default()

	//router.GET("/orders", api.GetOrders)
	router.POST("/orders/:id/pay", func(ctx *gin.Context) {
		api.PostOrderPayment(ctx, rdb, kafkaWriter)
	})

	fmt.Println(`backend-order running on port 8083`)

	router.Run("localhost:8083")
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
