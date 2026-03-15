docker exec -it cassandra cqlsh
USE ecommerce;

CREATE KEYSPACE IF NOT EXISTS ecommerce
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE inventory_items (
    id UUID,
    product_id UUID,
    stock_quantity int,
    last_updated timestamp,
    PRIMARY KEY (product_id)
);

CREATE TABLE cart_items (
    id UUID,
    cart_id UUID,
    product_id UUID,
    quantity int,
    PRIMARY KEY (cart_id)
);

INSERT INTO inventory_items (product_id, id, stock_quantity, last_updated)
VALUES (98b7f8ae-15bc-11f1-82eb-2cfda1bbb0fd,2e0086e5-e6ea-4174-968c-351a8f6a9c28,100,toTimestamp(now()));

CREATE TABLE IF NOT EXISTS users (
    user_id uuid PRIMARY KEY,
    username text,
    email text,
    password_hash text,
    created_at timestamp
);

CREATE INDEX IF NOT EXISTS ON users (email);