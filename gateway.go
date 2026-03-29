package main

import (
	"context"
	pb "ecommercePlatform/backend2/proto"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var counter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_counter",
		Help: "to count all the http requests",
	}, []string{"method", "status", "service"})

var timer = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_timer",
		Help:    "to calculate the time taken by the request",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "service"})

var rateLimited = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_rate_limited_total",
		Help: "Number of requests rejected by rate limiter",
	}, []string{"service", "reason"},
)

var cbState = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "gateway_circuit_breaker_state",
		Help: "Circuit breaker state per service (0=closed,1=open,2=half-open)",
	}, []string{"service"},
)

var proxyCanceledCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "gateway_proxy_canceled_total",
	Help: "Count of canceled proxied requests (client canceled).",
})

func init() {
	prometheus.MustRegister(counter, timer, rateLimited, cbState, proxyCanceledCounter)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

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
	Urls       []string `yaml:"urls"`
	Middleware []string `yaml:"middleware"`

	proxies []*httputil.ReverseProxy
	count   int64

	// Tuning values
	RateLimitRPS          float64 `yaml:"rate_limit_rps"`
	RateLimitBurst        float64 `yaml:"rate_limit_burst"`
	CBFailureThreshold    int     `yaml:"cb_failure_threshold"`
	CBResetTimeoutSeconds int     `yaml:"cb_reset_timeout_seconds"`
}

type MiddlewareWrapper func(next http.Handler) http.Handler

type TokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	last     time.Time
	rate     float64
	capacity float64
}

func NewTokenBucket(rate, capacity float64) *TokenBucket {
	return &TokenBucket{
		tokens:   capacity,
		last:     time.Now(),
		rate:     rate,
		capacity: capacity,
	}
}

func (tb *TokenBucket) TryConsume() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.last).Seconds()
	tb.last = now
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

type CBState int

const (
	CBClosed CBState = iota
	CBOpen
	CBHalfOpen
)

type CircuitBreaker struct {
	mu            sync.Mutex
	state         CBState
	failures      int
	lastFailure   time.Time
	failureThresh int
	resetTimeout  time.Duration
	halfProbes    int
}

func NewCircuitBreaker(failureThresh int, resetTimeoutSeconds int) *CircuitBreaker {
	if failureThresh <= 0 {
		failureThresh = 5
	}
	if resetTimeoutSeconds <= 0 {
		resetTimeoutSeconds = 10
	}
	return &CircuitBreaker{
		state:         CBClosed,
		failures:      0,
		failureThresh: failureThresh,
		resetTimeout:  time.Duration(resetTimeoutSeconds) * time.Second,
		halfProbes:    1,
	}
}

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	now := time.Now()

	switch cb.state {
	case CBClosed:
		return true
	case CBOpen:
		if now.Sub(cb.lastFailure) > cb.resetTimeout {
			cb.state = CBHalfOpen
			cb.halfProbes = 1
			return true
		}
		return false
	case CBHalfOpen:
		if cb.halfProbes > 0 {
			cb.halfProbes--
			return true
		}
		return false
	default:
		return false
	}
}

func (cb *CircuitBreaker) Report(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if success {
		cb.failures = 0
		cb.state = CBClosed
		return
	}
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.failureThresh {
		cb.state = CBOpen
	}
	if cb.state == CBHalfOpen {
		cb.state = CBOpen
	}
}

func (cb *CircuitBreaker) State() CBState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

type Gateway struct {
	Ser             map[string]*Service
	wrappers        map[string]MiddlewareWrapper
	apiKeys         map[string]bool
	rateLimiters    sync.Map
	CircuitBreakers map[string]*CircuitBreaker
	transport       *http.Transport
	// gRPC Client
	productClient pb.ProductServiceClient
	grpcConn      *grpc.ClientConn
}

func (g *Gateway) register(name string, wrapper MiddlewareWrapper) {
	g.wrappers[name] = wrapper
	log.Printf("Register a middleware %s", name)
}

// authAPIKeyMiddleware returns a middleware that checks the X-API-KEY header.
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

func (g *Gateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		serviceName := ""
		service := g.findService(r)
		if service != nil {
			serviceName = service.Name
		}
		next.ServeHTTP(recorder, r)
		duration := time.Since(startTime).Seconds()
		counter.WithLabelValues(r.Method, fmt.Sprintf("%d", recorder.statusCode), serviceName).Inc()
		timer.WithLabelValues(r.Method, serviceName).Observe(duration)
	})
}

func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := g.findService(r)

		apiKey := r.Header.Get("X-API-KEY")
		key := apiKey
		if key == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				key = r.RemoteAddr
			} else {
				key = host
			}
		}

		rate := 100.0
		burst := 200.0
		if service != nil {
			if service.RateLimitRPS > 0 {
				rate = service.RateLimitRPS
			}
			if service.RateLimitBurst > 0 {
				burst = service.RateLimitBurst
			}
		}

		mapKey := key
		if service != nil {
			mapKey = mapKey + ":" + service.Route
		}

		val, ok := g.rateLimiters.Load(mapKey)
		if !ok {
			tb := NewTokenBucket(rate, burst)
			g.rateLimiters.Store(mapKey, tb)
			val = tb
		}
		tb := val.(*TokenBucket)
		if !tb.TryConsume() {
			serviceName := "unknown"
			if service != nil {
				serviceName = service.Name
			}
			rateLimited.WithLabelValues(serviceName, "token_bucket").Inc()
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) circuitBreakerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		service := g.findService(r)
		if service == nil {
			next.ServeHTTP(w, r)
			return
		}
		cb, ok := g.CircuitBreakers[service.Route]
		if !ok {
			cb = NewCircuitBreaker(service.CBFailureThreshold, service.CBResetTimeoutSeconds)
			g.CircuitBreakers[service.Route] = cb
		}
		cbState.WithLabelValues(service.Name).Set(float64(cb.State()))

		if !cb.AllowRequest() {
			log.Printf("circuit open for service %s, rejection request", service.Name)
			http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
			return
		}
		rec := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(rec, r)
		if rec.statusCode >= 500 {
			cb.Report(false)
		} else {
			cb.Report(true)
		}
		cbState.WithLabelValues(service.Name).Set(float64(cb.State()))
	})
}

func (g *Gateway) correlationIdMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corId := r.Header.Get("X-Correlation-Id")
		if len(corId) == 0 {
			corId = uuid.NewString()
		}

		r.Header.Set("X-Correlation-Id", corId)
		w.Header().Set("X-Correlation-Id", corId)
		next.ServeHTTP(w, r)
	})
}

// NewGateway creates a Gateway from the provided Config.
// It builds the apiKeys map, registers built-in middleware and prepares reverse proxies for services.
func NewGateway(cfg *Config) *Gateway {
	conn, err := grpc.Dial("product-service:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		conn, err = grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: Could not connect to gRPC backend: %v", err)
		}
	}
	/*conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Warning: Could not connect to gRPC backend: %v", err)
	}*/

	sharedTransport := &http.Transport{
		MaxIdleConns:          20000,
		MaxIdleConnsPerHost:   5000,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	gw := &Gateway{
		Ser:             make(map[string]*Service),
		wrappers:        make(map[string]MiddlewareWrapper),
		apiKeys:         make(map[string]bool),
		CircuitBreakers: make(map[string]*CircuitBreaker),
		grpcConn:        conn,
		productClient:   pb.NewProductServiceClient(conn),
	}

	for _, key := range cfg.MiddlewareConfig.ApiKeyAuth.ValidKeys {
		gw.apiKeys[key] = true
	}
	gw.register("auth_apikey", gw.authAPIKeyMiddleware)
	gw.register("metrics", gw.metricsMiddleware)
	gw.register("rate_limit", gw.rateLimitMiddleware)
	gw.register("circuit_breaker", gw.circuitBreakerMiddleware)
	gw.register("correlation_id", gw.correlationIdMiddleware)

	for i := range cfg.Services {
		currSer := &cfg.Services[i]
		// Check if there is any url provided for the service
		if len(currSer.Urls) == 0 {
			log.Printf("Service %s does not have any urls defined", currSer.Name)
			return nil
		}
		for j := 0; j < len(currSer.Urls); j++ {
			target, err := url.Parse(currSer.Urls[j])
			if err != nil {
				log.Printf("invalid url %s for service %s: %v", currSer.Urls[j], currSer.Name, err)
				return nil
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.Transport = sharedTransport
			proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
				if err == context.Canceled {
					proxyCanceledCounter.Inc()
					return
				}
				log.Printf("proxy error: %v (backend=%s client=%s path=%s)", err, target.String(), req.RemoteAddr, req.URL.Path)
				http.Error(rw, "bad gateway", http.StatusBadGateway)
			}
			currSer.proxies = append(currSer.proxies, proxy)
		}
		gw.Ser[currSer.Route] = currSer
	}

	return gw
}

// findService looks up a Service by matching request path prefixes against registered routes.
func (g *Gateway) findService(r *http.Request) *Service {
	for route, service := range g.Ser {
		if strings.HasPrefix(r.URL.Path, route) {
			return service
		}
	}

	return nil
}

// NewProxy returns a reverse proxy for the service using round-robin load balancing.
func (s *Service) NewProxy() *httputil.ReverseProxy {
	newVal := atomic.AddInt64(&s.count, 1)

	index := newVal % int64(len(s.proxies))
	return s.proxies[index]
}

func (g *Gateway) handleGrpcSearch(w http.ResponseWriter, r *http.Request) {
	service := g.findService(r)
	if service == nil {
		http.Error(w, "gRPC client not initialized", http.StatusServiceUnavailable)
		return
	}

	cb, ok := g.CircuitBreakers[service.Route]
	if !ok {
		cb = NewCircuitBreaker(service.CBFailureThreshold, service.CBResetTimeoutSeconds)
		g.CircuitBreakers[service.Route] = cb
	}

	if !cb.AllowRequest() {
		log.Printf("CB Open: Blocking gRPC call to %s", service.Name)
		http.Error(w, "Service temporarily unavailable (CB Open)", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	query := r.URL.Query().Get("search")
	resp, err := g.productClient.SearchProducts(ctx, &pb.SearchRequest{
		Query: query,
		Page:  1,
		Limit: 10,
	})
	if err != nil {
		log.Printf("gRPC Error: %v", err)
		cb.Report(false)
		http.Error(w, "Internal Service Error", http.StatusInternalServerError)
		return
	}

	cb.Report(true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleGateway is the main HTTP handler for the gateway.
// It finds the target service for the request, wraps the service proxy with configured middleware (in reverse order),
// and forwards the request to the service proxy. If no service is found it returns 404.
func (g *Gateway) handleGateway(w http.ResponseWriter, r *http.Request) {
	log.Printf("Gateway received request for path: %s", r.URL.Path)
	service := g.findService(r)
	if service == nil {
		log.Printf("No service found for the url %s", r.URL.Path)
		http.Error(w, "No service found for this url", http.StatusNotFound)
		return
	}

	var curr http.Handler

	// Specific case for gRPC Search
	if r.URL.Path == "/product/products/v2" {
		curr = http.HandlerFunc(g.handleGrpcSearch)
	} else {
		curr = service.NewProxy()
	}

	for i := len(service.Middleware) - 1; i >= 0; i-- {
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
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:    cfg.Port,
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatalf("Error staring the server on port %s", cfg.Port)
	}
}
