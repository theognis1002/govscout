package db

import "database/sql"

type SavedSearchRow struct {
	ID              int64
	UserID          int64
	Name            string
	SearchQuery     *string
	NAICSCode       *string
	OppType         *string
	SetAside        *string
	State           *string
	Department      *string
	ActiveOnly      bool
	IncludeKeywords *string
	ExcludeKeywords *string
	MatchAll         bool
	NotifyEmail      *string
	ResponseDeadline *string
	Enabled          bool
	CreatedAt       string
	ModifiedAt      string
}

func CreateSavedSearch(db *sql.DB, s *SavedSearchRow) (int64, error) {
	result, err := db.Exec(`INSERT INTO saved_searches
		(user_id, name, search_query, naics_code, opp_type, set_aside, state, department,
		 active_only, include_keywords, exclude_keywords, match_all, notify_email, response_deadline, enabled)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.UserID, s.Name, s.SearchQuery, s.NAICSCode, s.OppType, s.SetAside, s.State, s.Department,
		boolToInt(s.ActiveOnly), s.IncludeKeywords, s.ExcludeKeywords, boolToInt(s.MatchAll),
		s.NotifyEmail, s.ResponseDeadline, boolToInt(s.Enabled))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateSavedSearch(db *sql.DB, s *SavedSearchRow) error {
	_, err := db.Exec(`UPDATE saved_searches SET
		name=?, search_query=?, naics_code=?, opp_type=?, set_aside=?, state=?, department=?,
		active_only=?, include_keywords=?, exclude_keywords=?, match_all=?, notify_email=?, response_deadline=?, enabled=?,
		modified_at=datetime('now')
		WHERE id=? AND user_id=?`,
		s.Name, s.SearchQuery, s.NAICSCode, s.OppType, s.SetAside, s.State, s.Department,
		boolToInt(s.ActiveOnly), s.IncludeKeywords, s.ExcludeKeywords, boolToInt(s.MatchAll),
		s.NotifyEmail, s.ResponseDeadline, boolToInt(s.Enabled),
		s.ID, s.UserID)
	return err
}

func GetSavedSearch(db *sql.DB, id int64) (*SavedSearchRow, error) {
	var s SavedSearchRow
	var activeOnly, matchAll, enabled int
	err := db.QueryRow(`SELECT id, user_id, name, search_query, naics_code, opp_type, set_aside,
		state, department, active_only, include_keywords, exclude_keywords, match_all,
		notify_email, response_deadline, enabled, created_at, modified_at
		FROM saved_searches WHERE id = ?`, id).Scan(
		&s.ID, &s.UserID, &s.Name, &s.SearchQuery, &s.NAICSCode, &s.OppType, &s.SetAside,
		&s.State, &s.Department, &activeOnly, &s.IncludeKeywords, &s.ExcludeKeywords, &matchAll,
		&s.NotifyEmail, &s.ResponseDeadline, &enabled, &s.CreatedAt, &s.ModifiedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.ActiveOnly = activeOnly == 1
	s.MatchAll = matchAll == 1
	s.Enabled = enabled == 1
	return &s, nil
}

func ListSavedSearches(db *sql.DB, userID int64) ([]SavedSearchRow, error) {
	rows, err := db.Query(`SELECT id, user_id, name, search_query, naics_code, opp_type, set_aside,
		state, department, active_only, include_keywords, exclude_keywords, match_all,
		notify_email, response_deadline, enabled, created_at, modified_at
		FROM saved_searches WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var searches []SavedSearchRow
	for rows.Next() {
		var s SavedSearchRow
		var activeOnly, matchAll, enabled int
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.Name, &s.SearchQuery, &s.NAICSCode, &s.OppType, &s.SetAside,
			&s.State, &s.Department, &activeOnly, &s.IncludeKeywords, &s.ExcludeKeywords, &matchAll,
			&s.NotifyEmail, &s.ResponseDeadline, &enabled, &s.CreatedAt, &s.ModifiedAt,
		); err != nil {
			return nil, err
		}
		s.ActiveOnly = activeOnly == 1
		s.MatchAll = matchAll == 1
		s.Enabled = enabled == 1
		searches = append(searches, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return searches, nil
}

func DeleteSavedSearch(db *sql.DB, id, userID int64) error {
	_, err := db.Exec("DELETE FROM saved_searches WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func SeedDefaultSearches(db *sql.DB, userID int64) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM saved_searches WHERE user_id = ?", userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	naics := "541511,541512,541513,541519,511210,518210"
	oppType := "o,p,k,r,s"
	includeKW := "redact, redaction, disclosure, FDO, foreign disclosure"
	deadline := "12m"

	defaults := []SavedSearchRow{
		{
			UserID:           userID,
			Name:             "Redaction & Disclosure — Software",
			NAICSCode:        &naics,
			OppType:          &oppType,
			IncludeKeywords:  &includeKW,
			ResponseDeadline: &deadline,
			ActiveOnly:       true,
			Enabled:          true,
		},
	}

	for _, s := range defaults {
		if _, err := CreateSavedSearch(db, &s); err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
