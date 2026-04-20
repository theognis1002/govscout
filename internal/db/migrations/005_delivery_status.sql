ALTER TABLE deliveries ADD COLUMN status TEXT NOT NULL DEFAULT 'sent';
ALTER TABLE deliveries ADD COLUMN attempts INTEGER NOT NULL DEFAULT 1;
ALTER TABLE deliveries ADD COLUMN last_attempted_at TEXT;
CREATE INDEX IF NOT EXISTS idx_deliveries_status ON deliveries(status, last_attempted_at);
