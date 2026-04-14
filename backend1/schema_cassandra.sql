--docker exec -it cassandra cqlsh

CREATE KEYSPACE IF NOT EXISTS ecommerce
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

USE ecommerce;

CREATE TABLE inventory_items (
    product_id UUID PRIMARY KEY,
    id UUID,
    stock_quantity int,
    last_updated timestamp
);

CREATE TABLE cart_items (
    cart_id UUID,
    product_id UUID,
    id UUID,
    quantity int,
    PRIMARY KEY (cart_id, product_id)
);

INSERT INTO inventory_items (product_id, id, stock_quantity, last_updated)
VALUES (98b7f8ae-15bc-11f1-82eb-2cfda1bbb0fd, 2e0086e5-e6ea-4174-968c-351a8f6a9c28, 100, toTimestamp(now()));
