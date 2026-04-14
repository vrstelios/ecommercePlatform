import http from 'k6/http';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// 1. Config: Set our"Load Profile"
export const options = {
    stages: [
        { duration: '30s', target: 50 },  // Ramp-up: from 0 in 50 users
        { duration: '1m', target: 50 },   // Stay: 50 users for 1 minute
        { duration: '30s', target: 100 }, // Spike: raise them 100
        { duration: '30s', target: 0 },   // Ramp-down:
    ],
    thresholds: {
        http_req_duration: ['p(95)<250', 'p(99)<400'], // 95%  req < 250ms, 99% < 400ms
        http_req_failed: ['rate<0.01'],               // Fewer errors from 1%
    },
};

const BASE_URL = 'http://localhost:8080'; 
const PARAMS = {
    headers: {
        'Content-Type': 'application/json',
        'X-API-KEY': 'abc-123',
    },
};

export default function () {
    const cartId = uuidv4();
    const productId = "f3dd7a7a-3512-11f1-b700-f6e91b152c83";

    // Step 1: Search Product
    let searchRes = http.get(`${BASE_URL}/product/products/v2?search=Laptop`, PARAMS);
    check(searchRes, { 'search status 200': (r) => r.status === 200 });

    sleep(1); // The user "thinks"

    // Step 2: Add to Cart
    let cartPayload = JSON.stringify({
        cartId: cartId,
        productId: productId,
        quantity: 1
    });
    let cartRes = http.post(`${BASE_URL}/api/cart/items`, cartPayload, PARAMS);
    check(cartRes, { 'add to cart status 200': (r) => r.status === 200 || r.status === 201 });

    sleep(1);

    // Step 3: Create Order
    let orderRes = http.post(`${BASE_URL}/api/orders/create/${cartId}`, cartPayload, PARAMS);
    check(orderRes, { 'order status 201': (r) => r.status === 201 || r.status === 200 });

    sleep(1);
}
