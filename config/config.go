package config

import (
	"encoding/json"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"os"
	"time"
)

type Config struct {
	Cassandra struct {
		Hosts    []string `json:"hosts"`
		Keyspace string   `json:"keyspace"`
		Port     int      `json:"port"`
	} `json:"cassandra"`
	Redis struct {
		Addr     string `json:"addr"`
		Password string `json:"password"`
		DB       int    `json:"db"`
	} `json:"redis"`
	Kafka struct {
		Broker string `json:"broker"`
		Topic  string `json:"topic"`
	} `json:"kafka"`
}

// Load Config
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var cfg Config
	err = json.NewDecoder(file).Decode(&cfg)
	return &cfg, err
}

// Connect-Cassandra
func (cfg *Config) ConnectCassandra() *gocql.Session {
	cluster := gocql.NewCluster(cfg.Cassandra.Hosts...)
	cluster.Keyspace = cfg.Cassandra.Keyspace
	cluster.Port = cfg.Cassandra.Port
	cluster.Consistency = gocql.Quorum
	session, err := cluster.CreateSession()
	if err != nil {
		fmt.Printf("Cassandra connection failed: %v\n", err)
		os.Exit(1)
	}
	return session
}

// Connect-Redis
func (cfg *Config) ConnectRedis() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
}

// GetKafkaWriter
func (cfg *Config) GetKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Broker),
		Topic:                  cfg.Kafka.Topic,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		Async:                  false,
		WriteTimeout:           10 * time.Second,
	}
}
