package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"os"
	"time"
)

const FilePath = "C:/Users/User/GolandProjects/ecommercePlatform/config/config.json"

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
		Broker        string `json:"broker"`
		TopicProducts string `json:"topic_products"`
		TopicPayments string `json:"topic_payments"`
	} `json:"kafka"`
	Database struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Name     string `json:"name"`
		SSLMode  string `json:"sslmode"`
	} `json:"database"`
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

// Connect-Postgres
func (cfg *Config) ConnectPostgres() *pgx.Conn {
	cnnDB := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Tehran",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.SSLMode)

	con, err := pgx.Connect(context.Background(), cnnDB)
	if err != nil {
		panic("Failed to connect to db: " + err.Error())
	}
	return con
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
func (cfg *Config) GetProductsKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Broker),
		Topic:                  cfg.Kafka.TopicProducts,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		Async:                  false,
		WriteTimeout:           10 * time.Second,
	}
}

func (cfg *Config) GetKafkaWriter() *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Kafka.Broker),
		Topic:                  cfg.Kafka.TopicPayments,
		Balancer:               &kafka.LeastBytes{},
		AllowAutoTopicCreation: true,
		Async:                  false,
		WriteTimeout:           10 * time.Second,
	}
}
