DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS carts;
DROP TABLE IF EXISTS products;

CREATE TABLE products (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    price       FLOAT NOT NULL);

CREATE TABLE carts (
    id        UUID PRIMARY KEY,
    user_id   UUID NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users_commerce(id) ON DELETE CASCADE
);

CREATE TABLE cart_items (
    id         UUID PRIMARY KEY,
    cart_id    UUID NOT NULL,
    product_id UUID NOT NULL,
    quantity   INT NOT NULL,
    FOREIGN KEY (cart_id) REFERENCES carts(id) ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE);

CREATE TABLE order_items (
    id             UUID PRIMARY KEY,
    product_id     UUID NOT NULL,
    order_id       UUID NOT NULL,
    quantity       INT NOT NULL,
    price_at_time  FLOAT NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(id)  ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE);

CREATE TABLE orders (
    id 			 UUID PRIMARY KEY,
    user_id 	 UUID NOT NULL,
    total_amount DECIMAL(10,2) NOT NULL,
    status 		 TEXT,
    FOREIGN KEY (user_id) REFERENCES users_commerce(id) ON DELETE CASCADE);

CREATE TABLE payments (
    id       UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    amount   FLOAT NOT NULL,
    status   TEXT,
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE);

CREATE TABLE inventory (
    id             UUID PRIMARY KEY,
    product_id     UUID NOT NULL,
    stock_quantity INT,
    last_updated   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE);
