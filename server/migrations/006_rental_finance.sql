CREATE TABLE IF NOT EXISTS rental_adjustments (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 adjustment_type TEXT NOT NULL CHECK (adjustment_type IN ('late_fee','damage_fee','discount','other')),
 amount NUMERIC(12,2) NOT NULL DEFAULT 0,
 note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS deposit_transactions (
 id BIGSERIAL PRIMARY KEY,
 rental_id BIGINT NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
 transaction_type TEXT NOT NULL CHECK (transaction_type IN ('hold','release','charge')),
 amount NUMERIC(12,2) NOT NULL DEFAULT 0,
 note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS rental_adjustments_rental_idx ON rental_adjustments(rental_id,id DESC);
CREATE INDEX IF NOT EXISTS deposit_transactions_rental_idx ON deposit_transactions(rental_id,id DESC);

UPDATE rentals SET final_total=GREATEST(COALESCE(final_total,amount),0) WHERE final_total IS NULL;

INSERT INTO deposit_transactions(rental_id,transaction_type,amount,note)
SELECT id,'hold',deposit,'migration snapshot'
FROM rentals
WHERE deposit > 0
  AND NOT EXISTS (SELECT 1 FROM deposit_transactions d WHERE d.rental_id=rentals.id);
