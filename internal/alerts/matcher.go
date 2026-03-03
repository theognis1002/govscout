package alerts

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/theognis1002/govscout/internal/db"
)

func RunMatcher(database *sql.DB) error {
	rows, err := database.Query(`SELECT id, user_id, name, search_query, naics_code, opp_type,
		set_aside, state, department, active_only, include_keywords, exclude_keywords,
		match_all, notify_email, response_deadline, enabled
		FROM saved_searches WHERE enabled = 1`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var searches []db.SavedSearchRow
	for rows.Next() {
		var s db.SavedSearchRow
		var activeOnly, matchAll, enabled int
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &s.SearchQuery, &s.NAICSCode, &s.OppType,
			&s.SetAside, &s.State, &s.Department, &activeOnly, &s.IncludeKeywords, &s.ExcludeKeywords,
			&matchAll, &s.NotifyEmail, &s.ResponseDeadline, &enabled); err != nil {
			return err
		}
		s.ActiveOnly = activeOnly == 1
		s.MatchAll = matchAll == 1
		s.Enabled = enabled == 1
		searches = append(searches, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, search := range searches {
		if err := matchSearch(database, search); err != nil {
			log.Printf("matcher error for search %d: %v", search.ID, err)
		}
	}
	return nil
}

func matchSearch(database *sql.DB, search db.SavedSearchRow) error {
	filters := buildFilters(search, 1000)

	result, err := db.ListOpportunities(database, filters)
	if err != nil {
		return err
	}

	includeKW := parseKeywords(deref(search.IncludeKeywords))
	excludeKW := parseKeywords(deref(search.ExcludeKeywords))

	alertCount := 0
	for _, opp := range result.Opportunities {
		text := strings.ToLower(deref(opp.Title) + " " + deref(opp.Description) + " " + deref(opp.Department))

		if !matchKeywords(text, includeKW, search.MatchAll) {
			continue
		}
		if excludeAny(text, excludeKW) {
			continue
		}

		result, err := db.InsertAlertIfNotExists(database, search.ID, opp.ID)
		if err != nil {
			log.Printf("insert alert error: %v", err)
			continue
		}
		if n, _ := result.RowsAffected(); n > 0 {
			alertCount++
		}
	}

	if alertCount > 0 {
		log.Printf("search %q: %d new alerts", search.Name, alertCount)
		deliverEmail(database, search)
	}
	return nil
}

func matchKeywords(text string, keywords []string, matchAll bool) bool {
	if len(keywords) == 0 {
		return true
	}
	if matchAll {
		for _, kw := range keywords {
			if !strings.Contains(text, kw) {
				return false
			}
		}
		return true
	}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func excludeAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func parseKeywords(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(strings.ToLower(k))
		if k != "" {
			result = append(result, k)
		}
	}
	return result
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// PreviewMatches returns opportunities that would match a saved search's criteria.
func PreviewMatches(database *sql.DB, search db.SavedSearchRow, limit int) ([]db.OpportunityListItem, error) {
	if limit <= 0 {
		limit = 20
	}
	filters := buildFilters(search, limit)

	result, err := db.ListOpportunities(database, filters)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	includeKW := parseKeywords(deref(search.IncludeKeywords))
	excludeKW := parseKeywords(deref(search.ExcludeKeywords))

	var matched []db.OpportunityListItem
	for _, opp := range result.Opportunities {
		text := strings.ToLower(deref(opp.Title) + " " + deref(opp.Description) + " " + deref(opp.Department))
		if !matchKeywords(text, includeKW, search.MatchAll) {
			continue
		}
		if excludeAny(text, excludeKW) {
			continue
		}
		matched = append(matched, opp)
	}
	return matched, nil
}

func buildFilters(search db.SavedSearchRow, limit int) db.ListFilters {
	f := db.ListFilters{
		NAICSCode:  deref(search.NAICSCode),
		OppType:    deref(search.OppType),
		SetAside:   deref(search.SetAside),
		State:      deref(search.State),
		Department: deref(search.Department),
		ActiveOnly: search.ActiveOnly,
		Limit:      limit,
	}
	if dl := deref(search.ResponseDeadline); dl != "" {
		f.ResponseDeadline = dl
		now := time.Now()
		f.ResponseDeadlineFrom = now.Format("01/02/2006")
		switch dl {
		case "1m":
			f.ResponseDeadlineTo = now.AddDate(0, 1, 0).Format("01/02/2006")
		case "3m":
			f.ResponseDeadlineTo = now.AddDate(0, 3, 0).Format("01/02/2006")
		case "6m":
			f.ResponseDeadlineTo = now.AddDate(0, 6, 0).Format("01/02/2006")
		case "12m":
			f.ResponseDeadlineTo = now.AddDate(1, 0, 0).Format("01/02/2006")
		}
	}
	return f
}
