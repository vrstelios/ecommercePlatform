package api

import (
	"context"
	"ecommercePlatform/backend3/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"net/http"
)

func PostOrderPayment(ctx *gin.Context, rdb *redis.Client, KafkaWriter *kafka.Writer) {
	orderId := ctx.Param("id")
	cartId := ctx.Query("cartId")
	var payment models.Payment
	err := ctx.ShouldBindJSON(&payment)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid json"})
		return
	}

	if cartId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "cartId query parameter is required for finalization"})
		return
	}

	exists, _ := rdb.Exists(ctx.Request.Context(), orderId).Result()
	if exists == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Order not found in cache"})
		return
	}

	status := "failed"
	if payment.PaymentMethod == "credit_card" {
		status = "completed"
	}

	// Create event for Kafka
	eventPayload, _ := json.Marshal(map[string]string{
		"order_id": orderId,
		"cart_id":  cartId,
		"status":   status,
	})

	err = KafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(orderId),
		Value: eventPayload,
	})
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to send event to Kafka"})
		return
	}

	ctx.JSON(200, gin.H{"status": status, "message": "Event sent to Kafka"})
}

func StartPaymentWorker(session *gocql.Session, rdb *redis.Client) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "payment-events",
		GroupID:  "order-group",
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	fmt.Println("Worker: Waiting for payment events...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Worker Error: %v\n", err)
			break
		}

		var event struct {
			OrderID string `json:"order_id"`
			CartID  string `json:"cart_id"`
			Status  string `json:"status"`
		}
		if err := json.Unmarshal(m.Value, &event); err != nil {
			fmt.Println("Worker: Error parsing message")
			continue
		}

		if event.Status == "completed" {
			fmt.Printf("Worker: Finalizing Order %s for Cart %s\n", event.OrderID, event.CartID)

			// --- CASSANDRA FINALIZE ---
			// Delete cart items from Cassandra
			deleteQuery := "DELETE FROM cart_items WHERE cart_id = ?"
			if err := session.Query(deleteQuery, event.CartID).Exec(); err != nil {
				fmt.Printf("Worker: Failed to delete cart %s: %v\n", event.CartID, err)
			} else {
				fmt.Printf("Worker: Successfully cleared cart %s from Cassandra\n", event.CartID)
			}

			// Redis Cleanup
			if err := rdb.Del(context.Background(), event.OrderID).Err(); err != nil {
				fmt.Printf("Worker: Failed to delete order %s from Redis: %v\n", event.OrderID, err)
			} else {
				fmt.Printf("Worker: Successfully cleaned up Redis for Order %s\n", event.OrderID)
			}
		}
	}
}
