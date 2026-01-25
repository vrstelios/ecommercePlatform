package main

import (
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port     string    `yaml:"port"`
	Services []Service `yaml:"services"`
}

type Service struct {
	Name  string `yaml:"name"`
	Route string `yaml:"route_pattern"`
	Url   string `yaml:"url"`
	proxy *httputil.ReverseProxy
}

type Gateway struct {
	Ser map[string]*Service
}

func NewGateway(cfg *Config) *Gateway {
	gw := &Gateway{
		Ser: make(map[string]*Service),
	}

	for i := range cfg.Services {
		currSer := &cfg.Services[i]

		target, err := url.Parse(currSer.Url)
		if err != nil {
			return nil
		}

		currSer.proxy = httputil.NewSingleHostReverseProxy(target)
		gw.Ser[currSer.Route] = currSer
	}

	return gw
}

func (g *Gateway) findService(r *http.Request) *Service {
	for route, service := range g.Ser {
		if strings.HasPrefix(r.URL.Path, route) {
			return service
		}
	}

	return nil
}

func (g *Gateway) handleGateway(w http.ResponseWriter, r *http.Request) {
	service := g.findService(r)
	if service == nil {
		log.Printf("No service found for the url %s", r.URL.Path)
		http.Error(w, "No service found for this url", http.StatusNotFound)
		return
	}

	log.Printf("Forwading the request of %s path to the service %s", r.URL.Path, service.Name)
	service.proxy.ServeHTTP(w, r)
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

func main() {
	cfg := loadConfig()

	gw := NewGateway(cfg)
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(gw.handleGateway))
	server := &http.Server{
		Addr:    cfg.Port,
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Error staring the server on port %s", cfg.Port)
	}
}
