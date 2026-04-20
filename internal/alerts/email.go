package alerts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/theognis1002/govscout/internal/db"
	"github.com/theognis1002/govscout/internal/samgov"
)

const (
	emailChannel     = "email"
	maxEmailAttempts = 5
	emailRetryAfter  = 2 * time.Hour
	emailSendTimeout = 30 * time.Second
)

// emailSender is the subset of resend.EmailsSvc used by this package — stubbable in tests.
type emailSender interface {
	SendWithContext(ctx context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
}

// senderFactory lets tests swap in a stub. Default uses Resend.
var senderFactory = func(apiKey string) emailSender {
	return resend.NewClient(apiKey).Emails
}

func deliverEmail(ctx context.Context, database *sql.DB, search db.SavedSearchRow) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return
	}
	recipients := parseRecipients(search.NotifyEmail)
	if len(recipients) == 0 {
		return
	}

	// 24h rate limit based on last *successful* delivery.
	lastDelivery, err := db.LastEmailDeliveryForSearch(database, search.ID)
	if err != nil {
		log.Printf("check last email delivery for search %d: %v", search.ID, err)
		return
	}
	if lastDelivery != nil && time.Since(*lastDelivery) < 24*time.Hour {
		log.Printf("email already sent today for search %d, skipping", search.ID)
		return
	}

	undelivered, err := db.UndeliveredAlertsForSearch(database, search.ID)
	if err != nil {
		log.Printf("list undelivered alerts for search %d: %v", search.ID, err)
		return
	}
	if len(undelivered) == 0 {
		return
	}

	sendAndRecord(ctx, database, search, recipients, undelivered, apiKey)
}

// RetryFailedEmails retries any 'failed' email deliveries that are due for another
// attempt (and marks exhausted ones 'abandoned'). Called once per RunMatcher cycle.
func RetryFailedEmails(ctx context.Context, database *sql.DB) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return
	}
	if err := db.AbandonExhaustedDeliveries(database, emailChannel, maxEmailAttempts); err != nil {
		log.Printf("abandon exhausted deliveries: %v", err)
	}
	due, err := db.FailedDeliveriesDue(database, emailChannel, maxEmailAttempts, emailRetryAfter)
	if err != nil {
		log.Printf("list failed deliveries: %v", err)
		return
	}
	if len(due) == 0 {
		return
	}

	// Group by saved_search_id.
	bySearch := map[int64][]db.AlertWithOpp{}
	for _, a := range due {
		bySearch[a.SavedSearchID] = append(bySearch[a.SavedSearchID], a)
	}

	for searchID, alerts := range bySearch {
		if err := ctx.Err(); err != nil {
			return
		}
		search, err := db.GetSavedSearch(database, searchID)
		if err != nil {
			log.Printf("load search %d for retry: %v", searchID, err)
			continue
		}
		recipients := parseRecipients(search.NotifyEmail)
		if len(recipients) == 0 {
			continue
		}
		log.Printf("retrying %d failed email deliveries for search %d", len(alerts), searchID)
		sendAndRecord(ctx, database, *search, recipients, alerts, apiKey)
	}
}

func sendAndRecord(ctx context.Context, database *sql.DB, search db.SavedSearchRow, recipients []string, alerts []db.AlertWithOpp, apiKey string) {
	body := buildHTML(search, alerts)
	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "GovScout <alerts@resend.dev>"
	}
	subject := fmt.Sprintf("GovScout: %s — %d new opportunities", search.Name, len(alerts))
	params := &resend.SendEmailRequest{
		From: fromEmail, To: recipients, Subject: subject, Html: body,
	}

	sendCtx, cancel := context.WithTimeout(ctx, emailSendTimeout)
	defer cancel()

	sender := senderFactory(apiKey)
	var sendResp *resend.SendEmailResponse
	err := samgov.Do(sendCtx, resendRetryPolicy(), func(ctx context.Context) error {
		r, err := sender.SendWithContext(ctx, params)
		if err != nil {
			if isTransient(err) {
				return samgov.Retryable(err)
			}
			return err
		}
		sendResp = r
		return nil
	})

	if err != nil {
		log.Printf("send email for search %d failed: %v", search.ID, err)
		errMsg := err.Error()
		for _, a := range alerts {
			if rerr := db.RecordDeliveryAttempt(database, a.ID, emailChannel, "failed", nil, &errMsg); rerr != nil {
				log.Printf("record failed delivery for alert %d: %v", a.ID, rerr)
			}
		}
		return
	}

	log.Printf("email sent for search %d: %s (%d alerts)", search.ID, sendResp.Id, len(alerts))
	status := 200
	for _, a := range alerts {
		if rerr := db.RecordDeliveryAttempt(database, a.ID, emailChannel, "sent", &status, nil); rerr != nil {
			log.Printf("record delivery for alert %d: %v", a.ID, rerr)
		}
	}
}

func parseRecipients(notify *string) []string {
	if notify == nil || *notify == "" {
		return nil
	}
	var out []string
	for _, e := range strings.Split(*notify, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

func buildHTML(search db.SavedSearchRow, alerts []db.AlertWithOpp) string {
	var body strings.Builder
	body.WriteString("<h2>")
	body.WriteString(html.EscapeString(search.Name))
	body.WriteString("</h2>")
	body.WriteString(fmt.Sprintf("<p>%d new matching opportunities:</p>", len(alerts)))
	body.WriteString("<table border='1' cellpadding='8' cellspacing='0' style='border-collapse:collapse'>")
	body.WriteString("<tr><th>Title</th><th>Type</th><th>Department</th><th>Posted</th><th>Link</th></tr>")
	for _, a := range alerts {
		title := "Untitled"
		if a.OppTitle != nil {
			title = *a.OppTitle
		}
		oppType := ""
		if a.OppType != nil {
			oppType = *a.OppType
		}
		dept := ""
		if a.Department != nil {
			dept = *a.Department
		}
		posted := ""
		if a.PostedDate != nil {
			posted = *a.PostedDate
		}
		link := fmt.Sprintf("https://sam.gov/opp/%s/view", a.OpportunityID)
		body.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><a href='%s'>View</a></td></tr>",
			html.EscapeString(title), html.EscapeString(oppType), html.EscapeString(dept), html.EscapeString(posted), html.EscapeString(link)))
	}
	body.WriteString("</table>")
	return body.String()
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	// Resend returns errors embedding HTTP status strings. Treat 5xx and 429 and
	// context-unrelated network errors as transient.
	s := err.Error()
	if strings.Contains(s, "429") || strings.Contains(s, "500") || strings.Contains(s, "502") ||
		strings.Contains(s, "503") || strings.Contains(s, "504") ||
		strings.Contains(s, "EOF") || strings.Contains(s, "connection") || strings.Contains(s, "timeout") {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}

func resendRetryPolicy() samgov.RetryPolicy {
	return samgov.RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    5 * time.Second,
		MaxElapsed:  20 * time.Second,
	}
}
