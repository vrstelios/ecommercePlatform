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
	Port             string      `yaml:"port"`
	Services         []Service   `yaml:"services"`
	MiddlewareConfig ConfigTypes `yaml:"middleware_config"`
}

type ConfigTypes struct {
	ApiKeyAuth ApiConfig `yaml:"auth_apikey"`
}

type ApiConfig struct {
	ValidKeys []string `yaml:"valid_keys"`
}

type Service struct {
	Name       string   `yaml:"name"`
	Route      string   `yaml:"route_pattern"`
	Url        string   `yaml:"url"`
	Middleware []string `yaml:"middleware"`

	proxy *httputil.ReverseProxy
}

type MiddlewareWrapper func(next http.Handler) http.Handler

type Gateway struct {
	Ser      map[string]*Service
	wrappers map[string]MiddlewareWrapper
	apiKeys  map[string]bool
}

func (g *Gateway) register(name string, wrapper MiddlewareWrapper) {
	g.wrappers[name] = wrapper
	log.Printf("Register a middleware %s", name)
}

func (g *Gateway) authAPIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if _, valid := g.apiKeys[apiKey]; !valid {
			log.Printf("Authentication failed, API KEY %s not found", apiKey)
			http.Error(w, "Authentication Failed", http.StatusUnauthorized)
			return
		}
		log.Printf("User authenticated")
		next.ServeHTTP(w, r)
	})
}

func NewGateway(cfg *Config) *Gateway {
	gw := &Gateway{
		Ser:      make(map[string]*Service),
		wrappers: make(map[string]MiddlewareWrapper),
		apiKeys:  make(map[string]bool),
	}

	for _, key := range cfg.MiddlewareConfig.ApiKeyAuth.ValidKeys {
		gw.apiKeys[key] = true
	}
	gw.register("auth_apikey", gw.authAPIKeyMiddleware)

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

	var curr http.Handler = service.proxy
	for i := len(service.Middleware) - 1; i == 0; i-- {
		middlewareName := service.Middleware[i]
		middleware, ok := g.wrappers[middlewareName]
		if !ok {
			log.Printf("middleware with name %s is not present", middlewareName)
			continue
		}
		curr = middleware(curr)
	}

	log.Printf("Forwading the request of %s path to the service %s with %d middlewares", r.URL.Path, service.Name, len(service.Middleware))
	curr.ServeHTTP(w, r)
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
