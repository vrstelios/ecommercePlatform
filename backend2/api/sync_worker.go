package api

import (
	"bytes"
	"context"
	"ecommercePlatform/backend2/models"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/segmentio/kafka-go"
)

func StartElasticSyncWorker(es *elasticsearch.Client, broker string, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   topic,
		GroupID: "elastic-sync-group",
	})
	defer reader.Close()

	fmt.Println("Elastic Sync Worker: Waiting for product updates...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Worker Error: %v\n", err)
			continue
		}
		fmt.Printf("Worker: Received message: %s\n", string(m.Value))

		var prod models.Products
		if err = json.Unmarshal(m.Value, &prod); err != nil {
			fmt.Println("Worker: Error parsing message")
			continue
		}

		data, err := json.Marshal(prod)
		if err != nil {
			fmt.Println("Worker: Error marshaling product data", err)
		}
		res, err := es.Index("products", bytes.NewReader(data), es.Index.WithDocumentID(prod.Id))
		if err == nil {
			fmt.Printf("Sync: Product %s indexed to Elasticsearch\n", prod.Id)
			res.Body.Close()
		}
	}
}
