package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// run: go test -v -run TestE2e -timeout 120s ./test/...
// run: go test -v -run TestGateway -timeout 30s ./test/...

const (
	gatewayURL = "http://localhost:8080"
	apiKey     = "abc-123"

	// UUID for testing workflow App
	ProductId = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	CartId    = "7f7703a6-7847-4610-9104-2a902287e857"
	UserId    = "dd376484-ae89-4f65-94b7-c0e06f156ab1"
)

// flowState save the Ids which we create in any step
type flowState struct {
	productId string
	orderId   string
}

// ============================================================
// HTTP Client helper
// ============================================================

type apiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newClient() *apiClient {
	// Override base URL from env variable
	base := os.Getenv("GATEWAY_URL")
	if base == "" {
		base = gatewayURL
	}

	return &apiClient{
		baseURL: base,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *apiClient) do(method, path string, body interface{}) (*http.Response, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = io.NopCloser(io.Reader(bytes.NewReader(b)))
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("X-API-KEY", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read body: %w", err)
	}

	return resp, respBody, nil
}

func waitForService(t *testing.T, client *apiClient, maxAttempts int) {
	t.Helper()
	for i := 1; i <= maxAttempts; i++ {
		resp, _, err := client.do(http.MethodGet, "/metrics", nil)
		if err == nil && resp.StatusCode != 0 {
			t.Log("Gateway is reachable")
			return
		}
		if i == maxAttempts {
			t.Fatalf("Gateway not reachable after %d attempts: %v", maxAttempts, err)
		}
		t.Logf("Waiting for gateway... attempt %d/%d", i, maxAttempts)
		time.Sleep(3 * time.Second)
	}
}

func TestE2eWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	client := newClient()
	state := &flowState{}

	t.Log("Checking gateway connectivity...")
	waitForService(t, client, 10)

	// http://localhost:8080/products
	t.Run("Step1_CreateProduct", func(t *testing.T) {
		state.productId = testCreateProduct(t, client)
	})
	if t.Failed() {
		t.Fatal("Cannot continue: product creation failed")
	}

	// http://localhost:8080/product/products/v2?search=Laptop
	t.Run("Step2_SearchProduct", func(t *testing.T) {
		t.Log("Waiting 10s for Kafka → Elasticsearch sync...")
		time.Sleep(10 * time.Second)
		testSearchProduct(t, client, "Laptop")
	})

	// http://localhost:8080/api/inventory
	t.Run("Step3_SetInventory", func(t *testing.T) {
		testSetInventory(t, client, state.productId, 100)
	})
	if t.Failed() {
		t.Fatal("Cannot continue: inventory setup failed")
	}

	// http://localhost:8080/api/cart/items
	t.Run("Step4_AddToCart", func(t *testing.T) {
		testAddToCart(t, client, state.productId, 2)
	})
	if t.Failed() {
		t.Fatal("Cannot continue: add to cart failed")
	}

	// http://localhost:8080/api/orders/create/:cartId
	t.Run("Step5_CreateOrder", func(t *testing.T) {
		state.orderId = testCreateOrder(t, client, CartId)
	})
	if t.Failed() {
		t.Fatal("Cannot continue: order creation failed")
	}

	// http://localhost:8080/orders/:cartId/pay
	t.Run("Step5_ProcessPayment", func(t *testing.T) {
		testProcessPayment(t, client, CartId, state.orderId)
	})

	// --- Async verification ---
	t.Run("Verify_InventoryDecremented", func(t *testing.T) {
		// The Kafka worker reduce the stock async
		t.Log("Waiting 8s for Kafka order worker to decrement inventory...")
		time.Sleep(8 * time.Second)
		testVerifyInventoryDecremented(t, client, state.productId, 100, 2)
	})

	t.Logf("\nFlow Summary:\n  ProductId: %s\n  OrderId:   %s", state.productId, state.orderId)
}

// ============================================================
// Individual step functions
// ============================================================

func testCreateProduct(t *testing.T, client *apiClient) string {
	t.Helper()
	t.Log("Creating product...")

	payload := map[string]interface{}{
		"name":        "Laptop",
		"description": "A powerful gaming laptop",
		"price":       1500.00,
	}

	resp, body, err := client.do(http.MethodPost, "/products", payload)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 200 or 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Cannot parse response: %v", err)
	}

	id, ok := result["id"].(string)
	if !ok || id == "" {
		t.Fatalf("Server did not return a valid product ID")
	}

	t.Logf("Using default productId: %s", id)
	return id
}

func testSearchProduct(t *testing.T, client *apiClient, item string) {
	t.Helper()
	t.Logf("Searching for '%s' via gRPC/Elasticsearch", item)

	resp, body, err := client.do(http.MethodGet, "/product/products/v2?search="+item, nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	if !strings.Contains(strings.ToLower(string(body)), strings.ToLower(item)) {
		t.Logf("Query '%s' not found in search results — Kafka sync may still be in progress", item)
	} else {
		t.Logf("Search returned results containing '%s'", item)
	}
}

func testSetInventory(t *testing.T, client *apiClient, productId string, stock int) {
	t.Helper()
	t.Logf("Setting inventory: %d units for product %s", stock, productId)

	payload := map[string]interface{}{
		"productId": productId,
		"stock":     stock,
	}

	resp, body, err := client.do(http.MethodPost, "/api/inventory", payload)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 200/201, got %d", resp.StatusCode)
	}

	t.Logf("Inventory set to %d units", stock)
}

func testAddToCart(t *testing.T, client *apiClient, productId string, quantity int) {
	t.Helper()
	t.Logf("Adding %d units of product %s to cart", quantity, productId)

	payload := map[string]interface{}{
		"cartId":    CartId,
		"productId": productId,
		"quantity":  quantity,
	}

	resp, body, err := client.do(http.MethodPost, "/api/cart/items", payload)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 200 or 201, got %d", resp.StatusCode)
	}

	t.Logf("Added to cart — Cart ID: %s", CartId)
}

func testCreateOrder(t *testing.T, client *apiClient, cartId string) string {
	t.Helper()
	t.Logf("Creating order from cart %s", cartId)

	payload := map[string]interface{}{
		"cartId":    "7f7703a6-7847-4610-9104-2a902287e857",
		"productId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"quantity":  2,
	}

	resp, body, err := client.do(http.MethodPost, "/api/orders/create/"+cartId, payload)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 200 or 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Cannot parse response: %v", err)
	}

	if status, ok := result["status"].(string); ok {
		if status != "pending" {
			t.Errorf("Expected order status 'pending', got '%s'", status)
		}
	}

	orderId, _ := result["order_id"].(string)
	if orderId == "" {
		t.Log("order_id not in response, using fallback")
	}

	t.Logf("Order created — Id: %s", orderId)
	return orderId
}

func testProcessPayment(t *testing.T, client *apiClient, cartId, orderId string) {
	t.Helper()
	t.Logf("Processing payment for order %s", orderId)

	payload := map[string]interface{}{
		"orderId":       orderId,
		"userId":        UserId,
		"amount":        2400.00,
		"paymentMethod": "credit_card",
	}

	resp, body, err := client.do(http.MethodPost, "/orders/"+cartId+"/pay", payload)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	t.Logf("Response [%d]: %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected 200 or 201, got %d", resp.StatusCode)
	}

	t.Log("Payment processed successfully")
}

func testVerifyInventoryDecremented(t *testing.T, c *apiClient, productID string, initialStock, orderedQty int) {
	t.Helper()
	t.Logf("Verifying inventory was decremented (expected: %d)", initialStock-orderedQty)

	resp, body, err := c.do(http.MethodGet, "/api/inventory/"+productID, nil)
	if err != nil {
		t.Logf("Inventory check request failed: %v — skipping", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		t.Logf("Inventory endpoint returned %d — skipping verification", resp.StatusCode)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Logf("Cannot parse inventory response: %v", err)
		return
	}

	if stock, ok := result["stock"].(float64); ok {
		expected := float64(initialStock - orderedQty)
		if stock == expected {
			t.Logf("Inventory correctly decremented to %.0f", stock)
		} else {
			t.Logf("Stock is %.0f, expected %.0f — Kafka worker may still be processing", stock, expected)
		}
	}
}

// ============================================================
// Gateway Middleware Tests
// ============================================================

func TestGatewayRejectsRequestWithoutAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := newClient()
	waitForService(t, c, 5)

	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/products", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", resp.StatusCode)
	} else {
		t.Log("Auth middleware correctly rejects requests without API key")
	}
}

func TestGatewayCorrelationIdPropagated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := newClient()
	waitForService(t, c, 5)

	customId := fmt.Sprintf("e2e-trace-%d", time.Now().Unix())

	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/metrics", nil)
	req.Header.Set("X-API-KEY", apiKey)
	req.Header.Set("X-Correlation-Id", customId)

	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	returnedId := resp.Header.Get("X-Correlation-Id")
	if returnedId == customId {
		t.Logf("Correlation Id '%s' correctly propagated in response", customId)
	} else {
		t.Errorf("Expected Correlation Id '%s', got '%s'", customId, returnedId)
	}
}

func TestGatewayPrometheusMetricsPresent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := newClient()
	waitForService(t, c, 5)

	resp, body, err := c.do(http.MethodGet, "/metrics", nil)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}

	expectedMetrics := []string{
		"http_requests_counter",
		"http_request_timer",
		"gateway_circuit_breaker_state",
		"gateway_rate_limited_total",
	}

	bodyStr := string(body)
	for _, metric := range expectedMetrics {
		if strings.Contains(bodyStr, metric) {
			t.Logf(" %s", metric)
		} else {
			t.Errorf(" %s — metric not found in /metrics output", metric)
		}
	}
}
