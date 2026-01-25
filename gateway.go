package main

import (
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type Config struct {
	port     string     `yaml:"port"`
	Services *[]Service `yaml:"services"`
}

type Service struct {
	name  string `yaml:"name"`
	route string `yaml:"route"`
	url   string `yaml:"url"`

	parsedURL url.URL
	proxy     *httputil.ReverseProxy
}

func loadConfig() *Config {
	var cfg Config
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("Error loading the config")
	}

	yaml.Unmarshal(data, &cfg)
	return &cfg
}

func newProxy(targetURL string) *httputil.ReverseProxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil
	}
	return httputil.NewSingleHostReverseProxy(target)
}

func main() {
	//cfg := loadConfig()
	backendURL := "http://localhost:8081"
	proxy := newProxy(backendURL)
	server := &http.Server{
		Addr:    ":8080",
		Handler: proxy,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting the gateway")
	}
}
