-- Rental workflow: statuses and notifications

CREATE TABLE IF NOT EXISTS rental_status_history (
    id BIGSERIAL PRIMARY KEY,
    rental_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Supported workflow statuses:
-- pending, approved, rejected, active, completed, cancelled
