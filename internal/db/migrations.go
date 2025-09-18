package db

import (
	"context"
)

const schema = `
create table if not exists orders (
    id serial primary key,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    number text unique not null,
    customer_name text not null,
    type text not null check (type in ('dine_in', 'takeout', 'delivery')),
    table_number integer,
    delivery_address text,
    total_amount decimal(10,2) not null,
    priority integer default 1,
    status text default 'received',
    processed_by text,
    completed_at timestamptz
);

create table if not exists order_items (
    id serial primary key,
    created_at timestamptz not null default now(),
    order_id integer references orders(id),
    name text not null,
    quantity integer not null,
    price decimal(8,2) not null
);

create table if not exists order_status_log (
    id serial primary key,
    created_at timestamptz not null default now(),
    order_id integer references orders(id),
    status text,
    changed_by text,
    changed_at timestamptz default current_timestamp,
    notes text
);

create table if not exists workers (
    id serial primary key,
    created_at timestamptz not null default now(),
    name text unique not null,
    type text not null,
    status text default 'online',
    last_seen timestamptz default current_timestamp,
    orders_processed integer default 0
);

-- sequence table for daily order numbers
create table if not exists order_counters (
    day date primary key,
    last_seq integer not null
);

create or replace function set_updated_at()
returns trigger as $$
begin
  new.updated_at = now();
  return new;
end;
$$ language plpgsql;

do $$ begin
  if not exists (select 1 from pg_trigger where tgname = 'orders_set_updated_at') then
    create trigger orders_set_updated_at before update on orders
    for each row execute function set_updated_at();
  end if;
end $$;
`

func RunMigrations(ctx context.Context, p *Pool) error {
	_, err := p.pool.Exec(ctx, schema)
	return err
}
