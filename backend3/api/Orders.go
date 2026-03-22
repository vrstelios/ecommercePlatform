package api

import (
	"ecommercePlatform/backend3/models"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"net/http"
)

func PostOrderPayment(ctx *gin.Context, rdb *redis.Client, KafkaWriter *kafka.Writer, logger *zap.Logger) {
	corId := ctx.GetHeader("X-Correlation-Id")
	cartId := ctx.Param("id")
	var payment models.Payment
	err := ctx.ShouldBindJSON(&payment)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid json"})
		return
	}

	if len(cartId) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "cartId query parameter is required for finalization"})
		return
	}

	logger.Info("Processing payment",
		zap.String("correlation_id", corId),
		zap.String("order_id", payment.OrderId),
		zap.Float64("amount", payment.Amount),
	)

	exists, _ := rdb.Exists(ctx.Request.Context(), payment.OrderId).Result()
	if exists == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Order not found in cache"})
		return
	}

	status := "failed"
	if payment.PaymentMethod == "credit_card" {
		status = "completed"
	}

	// Create event for Kafka
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"orderId": payment.OrderId,
		"cartId":  cartId,
		"userId":  payment.UserId,
		"total":   payment.Amount,
		"status":  status,
	})

	err = KafkaWriter.WriteMessages(ctx, kafka.Message{
		Key:   []byte(payment.OrderId),
		Value: eventPayload,
		Headers: []kafka.Header{
			{Key: "correlation_id", Value: []byte(ctx.GetHeader("correlation_id"))},
		},
	})
	if err != nil {
		ctx.JSON(500, gin.H{"error": "Failed to send event to Kafka", "Error": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{"status": status, "message": "Event sent to Kafka"})
}
