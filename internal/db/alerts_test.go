package db

import (
	"database/sql"
	"testing"
)

// setupAlertsTestDB opens a fresh in-memory DB, runs all migrations, and
// seeds one user, one saved_search, and two opportunities so that alert
// inserts have valid foreign keys to resolve against.
func setupAlertsTestDB(t *testing.T) (database *sql.DB, searchID int64, opp1, opp2 string) {
	t.Helper()
	// `:memory:` with the shared pragmas from Open would require a file path;
	// for tests we bypass Open() and run the migration SQL directly.
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })

	for i, m := range []string{migrationSQL, migration002SQL, migration003SQL, migration004SQL} {
		if _, err := d.Exec(m); err != nil {
			// Migration 002/003/004 add columns; on a fresh DB they should all apply
			// cleanly. If one fails with duplicate column it's fine.
			if !isDuplicateColumn(err) {
				t.Fatalf("migrate %d: %v", i+1, err)
			}
		}
	}

	res, err := d.Exec(`INSERT INTO users (username, password_hash) VALUES ('alice','x')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	uid, _ := res.LastInsertId()

	res, err = d.Exec(`INSERT INTO saved_searches (user_id, name) VALUES (?, 'my search')`, uid)
	if err != nil {
		t.Fatalf("seed saved_search: %v", err)
	}
	sid, _ := res.LastInsertId()

	opp1 = "opp-001"
	opp2 = "opp-002"
	for _, id := range []string{opp1, opp2} {
		if _, err := d.Exec(`INSERT INTO opportunities (id, title) VALUES (?, ?)`, id, "t-"+id); err != nil {
			t.Fatalf("seed opp %s: %v", id, err)
		}
	}
	return d, sid, opp1, opp2
}

func TestInsertAlertIfNotExists_FirstInsert(t *testing.T) {
	d, sid, opp1, _ := setupAlertsTestDB(t)

	res, err := InsertAlertIfNotExists(d, sid, opp1)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if n != 1 {
		t.Errorf("RowsAffected = %d, want 1 for first insert", n)
	}

	// Confirm the row is actually visible.
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM alerts WHERE saved_search_id=? AND opportunity_id=?`, sid, opp1).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("alert count = %d, want 1", count)
	}
}

func TestInsertAlertIfNotExists_DuplicateIsSilentlyIgnored(t *testing.T) {
	d, sid, opp1, _ := setupAlertsTestDB(t)

	if _, err := InsertAlertIfNotExists(d, sid, opp1); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second insert with the same (search_id, opportunity_id) must not fail
	// and must not create a second row — this is the "notify users only once"
	// contract that the whole alert system depends on.
	res, err := InsertAlertIfNotExists(d, sid, opp1)
	if err != nil {
		t.Fatalf("duplicate insert returned error: %v", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected: %v", err)
	}
	if n != 0 {
		t.Errorf("RowsAffected on duplicate = %d, want 0", n)
	}

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM alerts WHERE saved_search_id=? AND opportunity_id=?`, sid, opp1).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("alert count after duplicate = %d, want 1", count)
	}
}

func TestInsertAlertIfNotExists_DifferentOppsAreDistinctRows(t *testing.T) {
	// Guards against a regression where the UNIQUE constraint is accidentally
	// widened (e.g. UNIQUE(saved_search_id) alone) — which would collapse all
	// alerts for a search into one row.
	d, sid, opp1, opp2 := setupAlertsTestDB(t)

	for _, opp := range []string{opp1, opp2} {
		res, err := InsertAlertIfNotExists(d, sid, opp)
		if err != nil {
			t.Fatalf("insert %s: %v", opp, err)
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			t.Errorf("opp %s: RowsAffected = %d, want 1", opp, n)
		}
	}

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM alerts WHERE saved_search_id=?`, sid).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("alert count = %d, want 2 (one per distinct opp)", count)
	}
}
