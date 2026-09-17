ALTER TABLE rentals DROP CONSTRAINT IF EXISTS rentals_status_check;
ALTER TABLE rentals ADD CONSTRAINT rentals_status_check CHECK(status IN ('hold','pending','review','confirmed','preparing','active','returned','completed','cancelled','expired','rejected'));
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS pickup_at TIMESTAMPTZ;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS returned_at TIMESTAMPTZ;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS odometer_start INT;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS odometer_end INT;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS fuel_start INT;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS fuel_end INT;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS late_fee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS damage_fee NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS final_total NUMERIC(12,2) NOT NULL DEFAULT 0;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS extension_count INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS rental_events (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 actor_role TEXT NOT NULL DEFAULT 'system',
 event_type TEXT NOT NULL,
 from_status TEXT,
 to_status TEXT,
 payload JSONB NOT NULL DEFAULT '{}'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rental_events_rental_idx ON rental_events(rental_id,id DESC);

CREATE TABLE IF NOT EXISTS rental_extras (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 name TEXT NOT NULL,
 qty INT NOT NULL DEFAULT 1,
 unit_price NUMERIC(12,2) NOT NULL DEFAULT 0,
 total NUMERIC(12,2) NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rental_payments (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 payment_type TEXT NOT NULL DEFAULT 'rental',
 status TEXT NOT NULL DEFAULT 'paid',
 amount NUMERIC(12,2) NOT NULL DEFAULT 0,
 provider TEXT NOT NULL DEFAULT 'mock',
 transaction_ref TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rental_inspections (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 kind TEXT NOT NULL,
 mileage INT,
 fuel_level INT,
 notes TEXT NOT NULL DEFAULT '',
 photos JSONB NOT NULL DEFAULT '[]'::jsonb,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS rental_expenses (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 expense_type TEXT NOT NULL DEFAULT 'other',
 amount NUMERIC(12,2) NOT NULL DEFAULT 0,
 note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rental_extras_rental_idx ON rental_extras(rental_id);
CREATE INDEX IF NOT EXISTS rental_payments_rental_idx ON rental_payments(rental_id);
CREATE INDEX IF NOT EXISTS rental_inspections_rental_idx ON rental_inspections(rental_id);
CREATE INDEX IF NOT EXISTS rental_expenses_rental_idx ON rental_expenses(rental_id);

UPDATE rentals SET final_total=amount WHERE final_total=0;
INSERT INTO rental_events(rental_id,actor_role,event_type,to_status,payload)
SELECT id,'system','migration_snapshot',status,'{"source":"v0.6 migration"}'::jsonb
FROM rentals r WHERE NOT EXISTS (SELECT 1 FROM rental_events e WHERE e.rental_id=r.id);
