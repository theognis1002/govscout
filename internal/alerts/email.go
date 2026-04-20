package alerts

import (
	"database/sql"
	"fmt"
	"html"
	"log"
	"os"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/theognis1002/govscout/internal/db"
)

func deliverEmail(database *sql.DB, search db.SavedSearchRow) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return
	}
	if search.NotifyEmail == nil || *search.NotifyEmail == "" {
		return
	}

	// Parse comma-separated recipients
	var recipients []string
	for _, e := range strings.Split(*search.NotifyEmail, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			recipients = append(recipients, e)
		}
	}
	if len(recipients) == 0 {
		return
	}

	// Rate limit: one email per search per 24h
	lastDelivery, err := db.LastEmailDeliveryForSearch(database, search.ID)
	if err != nil {
		log.Printf("check last email delivery for search %d: %v", search.ID, err)
		return
	}
	if lastDelivery != nil && time.Since(*lastDelivery) < 24*time.Hour {
		log.Printf("email already sent today for search %d, skipping", search.ID)
		return
	}

	// Get undelivered alerts
	undelivered, err := db.UndeliveredAlertsForSearch(database, search.ID)
	if err != nil {
		log.Printf("list undelivered alerts for search %d: %v", search.ID, err)
		return
	}
	if len(undelivered) == 0 {
		return
	}

	// Build HTML email
	var body strings.Builder
	body.WriteString("<h2>")
	body.WriteString(html.EscapeString(search.Name))
	body.WriteString("</h2>")
	body.WriteString(fmt.Sprintf("<p>%d new matching opportunities:</p>", len(undelivered)))
	body.WriteString("<table border='1' cellpadding='8' cellspacing='0' style='border-collapse:collapse'>")
	body.WriteString("<tr><th>Title</th><th>Type</th><th>Department</th><th>Posted</th><th>Link</th></tr>")
	for _, a := range undelivered {
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

	fromEmail := os.Getenv("RESEND_FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "GovScout <alerts@resend.dev>"
	}

	subject := fmt.Sprintf("GovScout: %s — %d new opportunities", search.Name, len(undelivered))

	client := resend.NewClient(apiKey)
	params := &resend.SendEmailRequest{
		From:    fromEmail,
		To:      recipients,
		Subject: subject,
		Html:    body.String(),
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		log.Printf("send email for search %d: %v", search.ID, err)
		return
	}

	log.Printf("email sent for search %d: %s (%d alerts)", search.ID, sent.Id, len(undelivered))
	status := 200
	for _, a := range undelivered {
		if err := db.InsertDelivery(database, a.ID, "email", &status, nil); err != nil {
			log.Printf("record delivery for alert %d: %v", a.ID, err)
		}
	}
}
