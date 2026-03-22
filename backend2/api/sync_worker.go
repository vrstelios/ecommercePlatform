package api

import (
	"bytes"
	"context"
	"ecommercePlatform/backend2/models"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func StartElasticSyncWorker(es *elasticsearch.Client, broker string, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: "elastic-sync-group",
	})
	defer reader.Close()
	logger, _ := zap.NewProduction()

	logger.Info("Elastic Sync Worker: Waiting for product updates...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			logger.Error("Worker Error:", zap.Error(err))
			continue
		}
		logger.Info("Worker: Received message: ", zap.String("Value:", string(m.Value)))

		var prod models.Products
		if err = json.Unmarshal(m.Value, &prod); err != nil {
			logger.Error("Worker: Error parsing message")
			continue
		}

		data, err := json.Marshal(prod)
		if err != nil {
			logger.Error("Worker: Error marshaling product data", zap.Error(err))
		}
		res, err := es.Index("products", bytes.NewReader(data), es.Index.WithDocumentID(prod.Id))
		if err == nil {
			logger.Error(fmt.Sprintf("Sync: Product %s indexed to Elasticsearch", prod.Id))
			res.Body.Close()
		}
	}
}
