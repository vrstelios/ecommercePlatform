package main

import (
	"ecommercePlatform/backend3/api"
	"ecommercePlatform/config"
	"fmt"
	"github.com/gin-gonic/gin"
)

//Είναι σχεδιασμένος για δεδομένα «σε κίνηση». Διαχειρίζεται συνεχείς ροές πληροφοριών
//(streams) σε πραγματικό χρόνο. Δεν περιμένει να τον ρωτήσεις· σου «σπρώχνει» την
//πληροφορία τη στιγμή που συμβαίνει.

/*Ασύγχρονη Επικοινωνία με Kafka (Event-Driven)
Όταν ο χρήστης πατάει "Checkout", το Cart Service στέλνει ένα μήνυμα στον
Kafka: "Ο χρήστης X θέλει να αγοράσει αυτά τα 5 προϊόντα". Το Order Service
"ακούει" αυτό το μήνυμα και δημιουργεί την παραγγελία στη δική του βάση.*/

func main() {
	cfg, _ := config.LoadConfig("C:/Users/User/GolandProjects/ecommercePlatform/config/config.json")

	// Connect to Cassandra
	session := cfg.ConnectCassandra()
	defer session.Close()

	// Connect to Redis
	rdb := cfg.ConnectRedis()
	defer rdb.Close()

	// Start Kafka Worker in Background
	go api.StartPaymentWorker(session, rdb)

	kafkaWriter := cfg.GetKafkaWriter()

	router := gin.Default()

	router.POST("/orders/:id/pay", func(ctx *gin.Context) {
		api.PostOrderPayment(ctx, rdb, kafkaWriter)
	})

	fmt.Println(`backend-order running on port 8083`)

	router.Run("localhost:8083")
}
