CREATE TABLE IF NOT EXISTS car_expenses (
 id BIGSERIAL PRIMARY KEY,
 owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 car_id BIGINT NOT NULL REFERENCES cars(id) ON DELETE CASCADE,
 expense_type TEXT NOT NULL DEFAULT 'Другое',
 amount NUMERIC(12,2) NOT NULL DEFAULT 0 CHECK (amount >= 0),
 note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS car_expenses_car_idx ON car_expenses(car_id,created_at DESC,id DESC);
