-- KEY Taxi: driver profile and live availability
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_city TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_car_brand TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_car_model TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_plate TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_status TEXT NOT NULL DEFAULT 'busy';
ALTER TABLE users ADD COLUMN IF NOT EXISTS taxi_updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS users_taxi_city_status_idx ON users(taxi_city, taxi_status);
