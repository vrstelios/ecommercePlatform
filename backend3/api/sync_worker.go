package api

import (
	"context"
	"encoding/json"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"time"
)

// Woker receives payment events from Kafka and finalizes orders by cleaning up Cassandra and Redis
func StartPaymentWorker(session *gocql.Session, rdb *redis.Client, db *pgx.Conn, broker string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{broker},
		Topic:    "payment-events",
		GroupID:  "payment-finalizer-group",
		MinBytes: 10e3,
		MaxBytes: 10e6,
		MaxWait:  1 * time.Second,
	})
	defer reader.Close()
	logger, _ := zap.NewProduction()

	logger.Info("Worker: Waiting for payment events...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			logger.Error("Worker: Error reading message from Kafka", zap.Error(err))
			continue
		}
		logger.Info("Worker: Received message:", zap.String("", string(m.Value)))

		// Read header for monitoring
		var traceId string
		for _, h := range m.Headers {
			if h.Key == "correlation_id" {
				traceId = string(h.Value)
			}
		}

		msgLogger := logger.With(zap.String("trace_id", traceId))
		if err != nil {
			msgLogger.Error("Worker: Error reading message", zap.Error(err))
			continue
		}

		var event struct {
			OrderId string  `json:"orderId"`
			CartId  string  `json:"cartId"`
			UserId  string  `json:"userId"`
			Total   float64 `json:"total"`
			Status  string  `json:"status"`
		}
		if err = json.Unmarshal(m.Value, &event); err != nil {
			logger.Error("Worker: Error parsing message")
			continue
		}

		msgLogger.Info("Worker: Processing payment", zap.String("orderId", event.OrderId))

		if event.Status == "completed" {
			logger.Info("Worker:Finalizing Order for Cart", zap.String("OrderId:", event.OrderId), zap.String("CartId:", event.CartId))

			// --- Updated postgreSQL ---
			orderQuery := `INSERT INTO orders (id, user_id, total_amount, status) VALUES ($1, $2, $3, $4)`
			_, err = db.Exec(context.Background(), orderQuery, event.OrderId, event.UserId, event.Total, "completed")
			if err != nil {
				logger.Error("Postgres-Order Sync Error:", zap.Error(err))
			}

			paymentQuery := `INSERT INTO payments (id, order_id, amount, status) VALUES ($1, $2, $3, $4)`
			_, err = db.Exec(context.Background(), paymentQuery, uuid.NewString(), event.OrderId, event.Total, "success")
			if err != nil {
				logger.Error("Postgres-Payment Sync Error:", zap.Error(err))
			}

			// --- CASSANDRA FINALIZE ---
			// Delete cart items from Cassandra
			deleteQuery := "DELETE FROM cart_items WHERE cart_id = ?"
			if err := session.Query(deleteQuery, event.CartId).Exec(); err != nil {
				logger.Error("Worker: Failed to delete cart", zap.String("CartId", event.CartId), zap.Error(err))
			} else {
				logger.Info("Worker: Successfully cleared cart from Cassandra", zap.String("CartId", event.CartId))
			}

			// Redis Cleanup
			if err := rdb.Del(context.Background(), event.OrderId).Err(); err != nil {
				logger.Error("Worker: Failed to delete order %s from Redis:", zap.String("CartId", event.OrderId), zap.Error(err))
			} else {
				logger.Info("Worker: Successfully cleaned up Redis for Order", zap.String("OrderId:", event.OrderId))
			}
		}
	}
}
