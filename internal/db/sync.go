package db

import "database/sql"

type SyncRunRow struct {
	ID             int64
	StartedAt      string
	FinishedAt     *string
	Context        string
	PostedFrom     *string
	PostedTo       *string
	APICalls       int
	RecordsFetched int
	RateLimited    bool
	ErrorMessage   *string
}

func InsertSyncRun(db *sql.DB, ctx, from, to string, apiCalls, records int, rateLimited bool, errMsg *string) (int64, error) {
	result, err := db.Exec(`INSERT INTO sync_runs
		(context, posted_from, posted_to, api_calls, records_fetched, rate_limited, error_message, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		ctx, from, to, apiCalls, records, boolToInt(rateLimited), errMsg)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func ListSyncRuns(db *sql.DB, limit int) ([]SyncRunRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(`SELECT id, started_at, finished_at, context, posted_from, posted_to,
		api_calls, records_fetched, rate_limited, error_message
		FROM sync_runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []SyncRunRow
	for rows.Next() {
		var r SyncRunRow
		var rl int
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.FinishedAt, &r.Context, &r.PostedFrom, &r.PostedTo,
			&r.APICalls, &r.RecordsFetched, &rl, &r.ErrorMessage); err != nil {
			return nil, err
		}
		r.RateLimited = rl == 1
		runs = append(runs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return runs, nil
}

func GetSyncState(db *sql.DB, key string) (string, error) {
	var val string
	err := db.QueryRow("SELECT value FROM sync_state WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

func SetSyncState(db *sql.DB, key, value string) error {
	_, err := db.Exec("INSERT INTO sync_state (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, value)
	return err
}

func GetEarliestPostedDate(db *sql.DB) (string, error) {
	var val sql.NullString
	err := db.QueryRow(`SELECT posted_date FROM opportunities
		WHERE posted_date IS NOT NULL AND posted_date != ''
		ORDER BY substr(posted_date,7,4)||substr(posted_date,1,2)||substr(posted_date,4,2) ASC
		LIMIT 1`).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if val.Valid {
		return val.String, nil
	}
	return "", nil
}
