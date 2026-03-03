CREATE UNIQUE INDEX IF NOT EXISTS idx_deliveries_alert_channel ON deliveries(alert_id, webhook_url);
