ALTER TABLE rentals ADD COLUMN IF NOT EXISTS pickup_meeting_at TIMESTAMPTZ;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS pickup_meeting_location TEXT NOT NULL DEFAULT '';
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS return_meeting_at TIMESTAMPTZ;
ALTER TABLE rentals ADD COLUMN IF NOT EXISTS return_meeting_location TEXT NOT NULL DEFAULT '';
UPDATE rentals SET payment_status='unpaid' WHERE source='marketplace' AND payment_status='mock_paid';
