import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4, randomItem } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { Rate, Trend } from 'k6/metrics';

// Run Grafana-metrics-test: docker run --rm -v "C:/Users/User/GolandProjects/ecommercePlatform/tests/load-tests:/scripts" --network="host" grafana/k6 run /scripts/performance-script-v2.js

// Custom Metrics
export const errorRate = new Rate('errors');
export const checkoutDuration = new Trend('checkout_duration');
export const cartAddDuration = new Trend('cart_add_duration');

// 1. Config: Set our"Load Profile"
export const options = {
    scenarios: {
        browsing_users: {
            executor: 'constant-vus',
            vus: 60,
            duration: '2m',
            exec: 'browseProducts',
        },
        shopping_experience: {
            executor: 'ramping-vus',
            startVUs: 0,
            stages: [
                { duration: '30s', target: 30 }, // Warm up
                { duration: '1m', target: 30 },  // Steady load
                { duration: '30s', target: 0 },  // Ramp down
            ],
            exec: 'fullShoppingFlow',
        },
    },
    thresholds: {
        http_req_duration: ['p(95)<250'], // 95% requests under from 250ms
        errors: ['rate<0.01'],
        checkout_duration: ['p(95)<600'],
    },
};

const BASE_URL = 'http://localhost:8080';
const PARAMS = {
    headers: {
        'Content-Type': 'application/json',
        'X-API-KEY': 'abc-123',
    },
};

// SETUP PHASE: Create data before the test
export function setup() {
    const products = [
        { id: "56d05d4c-37da-11f1-84d1-c217bbed9607", name: "MacBook-Pro" },
        { id: "24c25a1d-37db-11f1-83ff-a666abb77868", name: "Keyboard" },
        { id: "83e88e52-37da-11f1-84d1-c217bbed9607", name: "Monitor" },
        { id: "94e0d429-37da-11f1-84d1-c217bbed9607", name: "Mouse" }
    ];

    console.log("Running test with pre-seeded products and inventory...");
    return { products: products };
}

// SCENARIO 1: Searching (Read-Heavy)
export function browseProducts(data) {
    const product = randomItem(data.products);

    let res = http.get(`${BASE_URL}/product/products/v2?search=${product.name}`, PARAMS);

    check(res, { 'search success': (r) => r.status === 200 }) || errorRate.add(1);
    sleep(Math.random() * 2);
}

// SCENARIO 2: Full User Flow (Search -> Add -> Checkout)
export function fullShoppingFlow(data) {
    const cartId = "7f7703a6-7847-4610-9104-2a902287e857";
    const product = randomItem(data.products);

    // Βήμα 1: Search
    http.get(`${BASE_URL}/product/products/v2?search=${product.name}`, PARAMS);
    sleep(1);

    // Βήμα 2: Add to Cart
    const cartStart = new Date();
    let cartRes = http.post(`${BASE_URL}/api/cart/items`, JSON.stringify({
        cartId: cartId,
        productId: product.id,
        quantity: 2,
    }), { ...PARAMS, tags: { name: 'AddToCart' } });

    let cartOk = check(cartRes, { 'cart success': (r) => r.status === 200 || r.status === 201 });

    if (cartOk) {
        cartAddDuration.add(new Date() - cartStart);
        sleep(1);

        // Checkout step
        const checkoutStart = new Date();
        let orderRes = http.post(`${BASE_URL}/api/orders/create/${cartId}`, null, {
            ...PARAMS,
            tags: { name: 'Checkout' }
        });

        check(orderRes, { 'order success': (r) => r.status === 201 || r.status === 200 }) || errorRate.add(1);
        checkoutDuration.add(new Date() - checkoutStart);
    } else {
        errorRate.add(1);
    }
    sleep(1);
}