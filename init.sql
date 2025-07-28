CREATE TABLE orders (
    id                serial        PRIMARY KEY,
    created_at        timestamptz   NOT NULL DEFAULT now(),
    updated_at        timestamptz   NOT NULL DEFAULT now(),
    number            text          UNIQUE NOT NULL,
    customer_name     text          NOT NULL CHECK (char_length(customer_name) BETWEEN 1 AND 100),
    type              text          NOT NULL CHECK (type IN ('dine-in', 'takeout', 'delivery')),
    table_number      integer       CHECK (char_length(table_number) BETWEEN 1 AND 100),
    delivery_address  text,
    total_amount      decimal(10,2) NOT NULL,
    priority          integer       DEFAULT 1,
    status            text          NOT NULL DEFAULT 'received',
    processed_by      text,
    completed_at      timestamptz
);

CREATE TABLE order_items (
    id          serial        PRIMARY KEY,
    created_at  timestamptz   NOT NULL DEFAULT now(),
    order_id    integer       REFERENCES orders(id) ON DELETE CASCADE,
    name        text          NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    quantity    integer       NOT NULL CHECK (quantity BETWEEN 1 AND 10),
    price       decimal(8,2)  NOT NULL CHECK (price BETWEEN 0.01 AND 999.99)
);

CREATE TABLE order_status_log (
    id          serial        PRIMARY KEY,
    created_at  timestamptz   NOT NULL DEFAULT now(),
    order_id    integer       NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status      text          NOT NULL CHECK (status IN ('received', 'cooking', 'ready', 'completed', 'cancelled')),
    changed_by  text,
    changed_at  timestamptz   DEFAULT current_timestamp,
    notes       text
);

CREATE TABLE workers (
    id                serial        PRIMARY KEY,
    created_at        timestamptz   NOT NULL DEFAULT now(),
    name              text          UNIQUE NOT NULL,
    type              text          NOT NULL,
    status            text          DEFAULT 'online',
    last_seen         timestamptz   DEFAULT current_timestamp,
    orders_processed  integer       DEFAULT 0
);

CREATE TABLE order_sequence (
    sequence_date date PRIMARY KEY,
    counter       integer NOT NULL
);

CREATE OR REPLACE FUNCTION log_init_order_status()
RETURNS trigger AS $$
BEGIN
  INSERT INTO order_status_log(order_id, status)
  VALUES (NEW.id, 'received');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_log_init_status
AFTER INSERT ON orders
FOR EACH ROW
EXECUTE FUNCTION log_init_order_status();
