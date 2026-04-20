package alerts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/resend/resend-go/v3"
	"github.com/theognis1002/govscout/internal/db"
)

type stubSender struct {
	calls int
	err   func(n int) error
}

func (s *stubSender) SendWithContext(ctx context.Context, _ *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	s.calls++
	if s.err != nil {
		if e := s.err(s.calls); e != nil {
			return nil, e
		}
	}
	return &resend.SendEmailResponse{Id: fmt.Sprintf("msg-%d", s.calls)}, nil
}

func withStubSender(t *testing.T, s *stubSender) {
	t.Helper()
	orig := senderFactory
	senderFactory = func(string) emailSender { return s }
	t.Cleanup(func() { senderFactory = orig })
	t.Setenv("RESEND_API_KEY", "test")
}

func openAlertsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

// seedOneAlert creates a user, saved search (with notify_email), opportunity, and a single alert row.
func seedOneAlert(t *testing.T, database *sql.DB, email string) (searchID, alertID int64, search db.SavedSearchRow) {
	t.Helper()
	res, err := database.Exec(`INSERT INTO users (username, password_hash, is_admin) VALUES ('u','h',0)`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := res.LastInsertId()

	res, err = database.Exec(`INSERT INTO saved_searches (user_id, name, notify_email, enabled) VALUES (?, 'test', ?, 1)`, userID, email)
	if err != nil {
		t.Fatal(err)
	}
	searchID, _ = res.LastInsertId()

	_, err = database.Exec(`INSERT INTO opportunities (id, title, active) VALUES ('opp1', 'Example', 1)`)
	if err != nil {
		t.Fatal(err)
	}

	res, err = db.InsertAlertIfNotExists(database, searchID, "opp1")
	if err != nil {
		t.Fatal(err)
	}
	alertID, _ = res.LastInsertId()

	got, err := db.GetSavedSearch(database, searchID)
	if err != nil {
		t.Fatal(err)
	}
	return searchID, alertID, *got
}

func TestDeliverEmail_FailureRecordsFailedStatus(t *testing.T) {
	database := openAlertsTestDB(t)
	sender := &stubSender{err: func(int) error { return errors.New("500 server error") }}
	withStubSender(t, sender)

	_, alertID, search := seedOneAlert(t, database, "a@b.com")

	deliverEmail(context.Background(), database, search)

	var status string
	var attempts int
	if err := database.QueryRow(`SELECT status, attempts FROM deliveries WHERE alert_id = ? AND webhook_url = 'email'`, alertID).
		Scan(&status, &attempts); err != nil {
		t.Fatalf("no delivery row recorded: %v", err)
	}
	if status != "failed" {
		t.Errorf("status=%q, want failed", status)
	}
	if attempts < 1 {
		t.Errorf("attempts=%d, want >=1", attempts)
	}
}

func TestDeliverEmail_Success(t *testing.T) {
	database := openAlertsTestDB(t)
	sender := &stubSender{}
	withStubSender(t, sender)

	_, alertID, search := seedOneAlert(t, database, "a@b.com")
	deliverEmail(context.Background(), database, search)

	var status string
	if err := database.QueryRow(`SELECT status FROM deliveries WHERE alert_id = ?`, alertID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Errorf("status=%q, want sent", status)
	}
}

func TestRetryFailedEmails_RetriesFailedThenSucceeds(t *testing.T) {
	database := openAlertsTestDB(t)
	failMode := true
	sender := &stubSender{err: func(int) error {
		if failMode {
			return errors.New("500 transient")
		}
		return nil
	}}
	withStubSender(t, sender)

	_, alertID, search := seedOneAlert(t, database, "a@b.com")

	// Phase 1: sender always fails → delivery recorded as 'failed' after in-call retries.
	deliverEmail(context.Background(), database, search)
	var status string
	database.QueryRow(`SELECT status FROM deliveries WHERE alert_id = ?`, alertID).Scan(&status)
	if status != "failed" {
		t.Fatalf("pre-retry status=%q, want failed", status)
	}

	// Bypass the retry-gap window for the test by backdating last_attempted_at.
	if _, err := database.Exec(`UPDATE deliveries SET last_attempted_at = '2000-01-01 00:00:00' WHERE alert_id = ?`, alertID); err != nil {
		t.Fatal(err)
	}

	// Phase 2: sender succeeds. RetryFailedEmails should pick up the failed row.
	failMode = false
	RetryFailedEmails(context.Background(), database)

	database.QueryRow(`SELECT status FROM deliveries WHERE alert_id = ?`, alertID).Scan(&status)
	if status != "sent" {
		t.Errorf("post-retry status=%q, want sent", status)
	}
}

func TestRetryFailedEmails_AbandonsAfterMaxAttempts(t *testing.T) {
	database := openAlertsTestDB(t)
	sender := &stubSender{err: func(int) error { return errors.New("500 boom") }}
	withStubSender(t, sender)

	_, alertID, _ := seedOneAlert(t, database, "a@b.com")

	// Seed a delivery already at max attempts.
	if _, err := database.Exec(`INSERT INTO deliveries (alert_id, webhook_url, status, attempts, error_message, last_attempted_at)
		VALUES (?, 'email', 'failed', ?, 'err', '2000-01-01 00:00:00')`, alertID, maxEmailAttempts); err != nil {
		t.Fatal(err)
	}

	RetryFailedEmails(context.Background(), database)

	var status string
	database.QueryRow(`SELECT status FROM deliveries WHERE alert_id = ?`, alertID).Scan(&status)
	if status != "abandoned" {
		t.Errorf("status=%q, want abandoned", status)
	}
	if sender.calls != 0 {
		t.Errorf("should not have attempted send; calls=%d", sender.calls)
	}
}

func TestParseRecipients(t *testing.T) {
	one := "a@b.com, , c@d.com,"
	got := parseRecipients(&one)
	if len(got) != 2 || got[0] != "a@b.com" || got[1] != "c@d.com" {
		t.Errorf("got %v", got)
	}
	if r := parseRecipients(nil); r != nil {
		t.Errorf("nil notify should yield nil recipients, got %v", r)
	}
	empty := ""
	if r := parseRecipients(&empty); r != nil {
		t.Errorf("empty notify should yield nil, got %v", r)
	}
}

func TestIsTransient(t *testing.T) {
	cases := map[string]bool{
		"500 Internal Server Error":    true,
		"429 Too Many Requests":        true,
		"502 Bad Gateway":              true,
		"dial tcp: connection refused": true,
		"i/o timeout":                  true,
		"401 Unauthorized":             false,
		"invalid to email":             false,
	}
	for msg, want := range cases {
		if got := isTransient(errors.New(msg)); got != want {
			t.Errorf("isTransient(%q)=%v want %v", msg, got, want)
		}
	}
}
