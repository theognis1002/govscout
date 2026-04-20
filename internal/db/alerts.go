package db

import (
	"database/sql"
	"time"
)

type AlertRow struct {
	ID            int64
	SavedSearchID int64
	OpportunityID string
	CreatedAt     string
}

type AlertWithOpp struct {
	AlertRow
	OppTitle   *string
	OppType    *string
	PostedDate *string
	Department *string
	SearchName string
}

type DeliveryRow struct {
	ID          int64
	AlertID     int64
	Channel     string
	StatusCode  *int
	Error       *string
	DeliveredAt string
}

func InsertAlertIfNotExists(db *sql.DB, savedSearchID int64, opportunityID string) (sql.Result, error) {
	return db.Exec(
		"INSERT OR IGNORE INTO alerts (saved_search_id, opportunity_id) VALUES (?, ?)",
		savedSearchID, opportunityID)
}

func ListAlertsForSearch(db *sql.DB, searchID int64, limit, offset int) ([]AlertWithOpp, int64, error) {
	var total int64
	if err := db.QueryRow("SELECT COUNT(*) FROM alerts WHERE saved_search_id = ?", searchID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`SELECT a.id, a.saved_search_id, a.opportunity_id, a.created_at,
		o.title, o.opp_type, o.posted_date, o.department, ss.name
		FROM alerts a
		JOIN opportunities o ON o.id = a.opportunity_id
		JOIN saved_searches ss ON ss.id = a.saved_search_id
		WHERE a.saved_search_id = ?
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`, searchID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []AlertWithOpp
	for rows.Next() {
		var a AlertWithOpp
		if err := rows.Scan(&a.ID, &a.SavedSearchID, &a.OpportunityID, &a.CreatedAt,
			&a.OppTitle, &a.OppType, &a.PostedDate, &a.Department, &a.SearchName); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}

func ListAlertsForUser(db *sql.DB, userID int64, limit, offset int) ([]AlertWithOpp, int64, error) {
	var total int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM alerts a
		JOIN saved_searches ss ON ss.id = a.saved_search_id
		WHERE ss.user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := db.Query(`SELECT a.id, a.saved_search_id, a.opportunity_id, a.created_at,
		o.title, o.opp_type, o.posted_date, o.department, ss.name
		FROM alerts a
		JOIN opportunities o ON o.id = a.opportunity_id
		JOIN saved_searches ss ON ss.id = a.saved_search_id
		WHERE ss.user_id = ?
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []AlertWithOpp
	for rows.Next() {
		var a AlertWithOpp
		if err := rows.Scan(&a.ID, &a.SavedSearchID, &a.OpportunityID, &a.CreatedAt,
			&a.OppTitle, &a.OppType, &a.PostedDate, &a.Department, &a.SearchName); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return alerts, total, nil
}

func InsertDelivery(db *sql.DB, alertID int64, channel string, statusCode *int, errMsg *string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO deliveries (alert_id, webhook_url, status_code, error_message, status, attempts, last_attempted_at) VALUES (?,?,?,?,'sent',1,datetime('now'))",
		alertID, channel, statusCode, errMsg)
	return err
}

// RecordDeliveryAttempt inserts (or updates) a delivery row reflecting a send attempt.
// status should be "sent", "failed", or "abandoned".
func RecordDeliveryAttempt(db *sql.DB, alertID int64, channel, status string, statusCode *int, errMsg *string) error {
	_, err := db.Exec(`INSERT INTO deliveries (alert_id, webhook_url, status_code, error_message, status, attempts, last_attempted_at)
		VALUES (?,?,?,?,?,1,datetime('now'))
		ON CONFLICT(alert_id, webhook_url) DO UPDATE SET
			status=excluded.status,
			status_code=excluded.status_code,
			error_message=excluded.error_message,
			attempts=attempts+1,
			last_attempted_at=datetime('now')`,
		alertID, channel, statusCode, errMsg, status)
	return err
}

// FailedDeliveriesDue lists deliveries in status='failed' that are ready for retry
// (last_attempted_at older than retryAfter or null) and haven't exceeded maxAttempts.
func FailedDeliveriesDue(database *sql.DB, channel string, maxAttempts int, retryAfter time.Duration) ([]AlertWithOpp, error) {
	cutoff := time.Now().Add(-retryAfter).UTC().Format("2006-01-02 15:04:05")
	rows, err := database.Query(`SELECT a.id, a.saved_search_id, a.opportunity_id, a.created_at,
		o.title, o.opp_type, o.posted_date, o.department, ss.name
		FROM alerts a
		JOIN opportunities o ON o.id = a.opportunity_id
		JOIN saved_searches ss ON ss.id = a.saved_search_id
		JOIN deliveries d ON d.alert_id = a.id AND d.webhook_url = ?
		WHERE d.status = 'failed'
		  AND d.attempts < ?
		  AND (d.last_attempted_at IS NULL OR d.last_attempted_at <= ?)
		ORDER BY a.created_at DESC LIMIT 500`, channel, maxAttempts, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertWithOpp
	for rows.Next() {
		var a AlertWithOpp
		if err := rows.Scan(&a.ID, &a.SavedSearchID, &a.OpportunityID, &a.CreatedAt,
			&a.OppTitle, &a.OppType, &a.PostedDate, &a.Department, &a.SearchName); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AbandonExhaustedDeliveries marks as 'abandoned' any failed deliveries that
// have reached maxAttempts.
func AbandonExhaustedDeliveries(db *sql.DB, channel string, maxAttempts int) error {
	_, err := db.Exec(`UPDATE deliveries SET status='abandoned'
		WHERE webhook_url = ? AND status = 'failed' AND attempts >= ?`, channel, maxAttempts)
	return err
}

func LastEmailDeliveryForSearch(database *sql.DB, searchID int64) (*time.Time, error) {
	var deliveredAt string
	err := database.QueryRow(`SELECT d.delivered_at FROM deliveries d
		JOIN alerts a ON a.id = d.alert_id
		WHERE a.saved_search_id = ? AND d.webhook_url = 'email' AND d.status = 'sent'
		ORDER BY d.delivered_at DESC LIMIT 1`, searchID).Scan(&deliveredAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", deliveredAt, time.UTC)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func UndeliveredAlertsForSearch(database *sql.DB, searchID int64) ([]AlertWithOpp, error) {
	rows, err := database.Query(`SELECT a.id, a.saved_search_id, a.opportunity_id, a.created_at,
		o.title, o.opp_type, o.posted_date, o.department, ss.name
		FROM alerts a
		JOIN opportunities o ON o.id = a.opportunity_id
		JOIN saved_searches ss ON ss.id = a.saved_search_id
		LEFT JOIN deliveries d ON d.alert_id = a.id
		WHERE a.saved_search_id = ? AND d.id IS NULL
		ORDER BY a.created_at DESC LIMIT 200`, searchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []AlertWithOpp
	for rows.Next() {
		var a AlertWithOpp
		if err := rows.Scan(&a.ID, &a.SavedSearchID, &a.OpportunityID, &a.CreatedAt,
			&a.OppTitle, &a.OppType, &a.PostedDate, &a.Department, &a.SearchName); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}
