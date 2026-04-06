--docker exec -it postgres psql -U postgres

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id           UUID PRIMARY KEY,
    user_id      UUID NOT NULL,
    total_amount DECIMAL(10,2) NOT NULL,
    status       TEXT,
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS payments (
    id       UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    amount   FLOAT NOT NULL,
    status   TEXT,
    CONSTRAINT fk_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    price       FLOAT NOT NULL
);

INSERT INTO users (id, name, email, password_hash)
VALUES ('dd376484-ae89-4f65-94b7-c0e06f156ab1', 'Verros', 'verros@test.com', '$2a$10$TxZ8mwo.sap2LxP5AqfsDuY0Nr4uQZGpuVNFZAq9EevDsvfeD3LXy')
ON CONFLICT (id) DO NOTHING;