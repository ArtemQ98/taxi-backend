CREATE TABLE IF NOT EXISTS car_photos (
  id BIGSERIAL PRIMARY KEY,
  car_id BIGINT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
  url TEXT NOT NULL,
  filename TEXT NOT NULL DEFAULT '',
  is_primary BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS car_photos_car_idx ON car_photos(car_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS car_photos_primary_idx ON car_photos(car_id) WHERE is_primary;
