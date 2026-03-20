package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"time"
)

// Woker receives payment events from Kafka and finalizes orders by cleaning up Cassandra and Redis
func StartPaymentWorker(session *gocql.Session, rdb *redis.Client, db *pgx.Conn) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{"127.0.0.1:9092"},
		Topic:    "payment-events",
		GroupID:  "payment-finalizer-group",
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})
	defer reader.Close()

	fmt.Println("Worker: Waiting for payment events...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Worker Error: %v\n", err)
			continue
		}
		fmt.Printf("Worker: Received message: %s\n", string(m.Value))

		var event struct {
			OrderId string  `json:"orderId"`
			CartId  string  `json:"cartId"`
			UserId  string  `json:"userId"`
			Total   float64 `json:"total"`
			Status  string  `json:"status"`
		}
		if err = json.Unmarshal(m.Value, &event); err != nil {
			fmt.Println("Worker: Error parsing message")
			continue
		}

		if event.Status == "completed" {
			fmt.Printf("Worker: Finalizing Order %s for Cart %s\n", event.OrderId, event.CartId)

			// --- Updated postgreSQL ---
			orderQuery := `INSERT INTO orders (id, user_id, total_amount, status) VALUES ($1, $2, $3, $4)`
			_, err := db.Exec(context.Background(), orderQuery, event.OrderId, event.UserId, event.Total, "completed")
			if err != nil {
				fmt.Printf("Postgres-Order Sync Error: %v\n", err)
			}

			paymentQuery := `INSERT INTO payments (id, order_id, amount, status) VALUES ($1, $2, $3, $4)`
			_, err = db.Exec(context.Background(), paymentQuery, uuid.NewString(), event.OrderId, event.Total, "success")
			if err != nil {
				fmt.Printf("Postgres-Payment Sync Error: %v\n", err)
			}

			// --- CASSANDRA FINALIZE ---
			// Delete cart items from Cassandra
			deleteQuery := "DELETE FROM cart_items WHERE cart_id = ?"
			if err := session.Query(deleteQuery, event.CartId).Exec(); err != nil {
				fmt.Printf("Worker: Failed to delete cart %s: %v\n", event.CartId, err)
			} else {
				fmt.Printf("Worker: Successfully cleared cart %s from Cassandra\n", event.CartId)
			}

			// Redis Cleanup
			if err := rdb.Del(context.Background(), event.OrderId).Err(); err != nil {
				fmt.Printf("Worker: Failed to delete order %s from Redis: %v\n", event.OrderId, err)
			} else {
				fmt.Printf("Worker: Successfully cleaned up Redis for Order %s\n", event.OrderId)
			}
		}
	}
}
