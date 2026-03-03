package db

import "database/sql"

type SavedFilterRow struct {
	ID               int64
	UserID           int64
	Name             string
	SearchQuery      *string
	NAICSCode        *string
	OppType          *string
	SetAside         *string
	State            *string
	Department       *string
	ActiveOnly       bool
	ResponseDeadline *string
	IsDefault        bool
	SortOrder        int
	CreatedAt        string
	ModifiedAt       string
}

func CreateSavedFilter(db *sql.DB, f *SavedFilterRow) (int64, error) {
	result, err := db.Exec(`INSERT INTO saved_filters
		(user_id, name, search_query, naics_code, opp_type, set_aside, state, department,
		 active_only, response_deadline, is_default, sort_order)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.UserID, f.Name, f.SearchQuery, f.NAICSCode, f.OppType, f.SetAside, f.State, f.Department,
		boolToInt(f.ActiveOnly), f.ResponseDeadline, boolToInt(f.IsDefault), f.SortOrder)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func UpdateSavedFilter(db *sql.DB, f *SavedFilterRow) error {
	_, err := db.Exec(`UPDATE saved_filters SET
		name=?, search_query=?, naics_code=?, opp_type=?, set_aside=?, state=?, department=?,
		active_only=?, response_deadline=?, sort_order=?, modified_at=datetime('now')
		WHERE id=? AND user_id=?`,
		f.Name, f.SearchQuery, f.NAICSCode, f.OppType, f.SetAside, f.State, f.Department,
		boolToInt(f.ActiveOnly), f.ResponseDeadline, f.SortOrder,
		f.ID, f.UserID)
	return err
}

func GetSavedFilter(db *sql.DB, id int64) (*SavedFilterRow, error) {
	var f SavedFilterRow
	var activeOnly, isDefault int
	err := db.QueryRow(`SELECT id, user_id, name, search_query, naics_code, opp_type, set_aside,
		state, department, active_only, response_deadline, is_default, sort_order, created_at, modified_at
		FROM saved_filters WHERE id = ?`, id).Scan(
		&f.ID, &f.UserID, &f.Name, &f.SearchQuery, &f.NAICSCode, &f.OppType, &f.SetAside,
		&f.State, &f.Department, &activeOnly, &f.ResponseDeadline, &isDefault, &f.SortOrder,
		&f.CreatedAt, &f.ModifiedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.ActiveOnly = activeOnly == 1
	f.IsDefault = isDefault == 1
	return &f, nil
}

func ListSavedFilters(db *sql.DB, userID int64) ([]SavedFilterRow, error) {
	rows, err := db.Query(`SELECT id, user_id, name, search_query, naics_code, opp_type, set_aside,
		state, department, active_only, response_deadline, is_default, sort_order, created_at, modified_at
		FROM saved_filters WHERE user_id = ? ORDER BY sort_order, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var filters []SavedFilterRow
	for rows.Next() {
		var f SavedFilterRow
		var activeOnly, isDefault int
		if err := rows.Scan(
			&f.ID, &f.UserID, &f.Name, &f.SearchQuery, &f.NAICSCode, &f.OppType, &f.SetAside,
			&f.State, &f.Department, &activeOnly, &f.ResponseDeadline, &isDefault, &f.SortOrder,
			&f.CreatedAt, &f.ModifiedAt,
		); err != nil {
			return nil, err
		}
		f.ActiveOnly = activeOnly == 1
		f.IsDefault = isDefault == 1
		filters = append(filters, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return filters, nil
}

func DeleteSavedFilter(db *sql.DB, id, userID int64) error {
	_, err := db.Exec("DELETE FROM saved_filters WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func SeedDefaultFilters(db *sql.DB, userID int64) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM saved_filters WHERE user_id = ?", userID).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	naics := "541511,541512"
	deadline := "3m"
	setAsideSB := "SBA,8A,WOSB,SDVOSB,HUBZone"
	setAsideSDVOSB := "SDVOSB"

	defaults := []SavedFilterRow{
		{
			UserID:           userID,
			Name:             "Software Contracts (90 days)",
			NAICSCode:        &naics,
			ResponseDeadline: &deadline,
			ActiveOnly:       true,
			IsDefault:        true,
			SortOrder:        1,
		},
		{
			UserID:           userID,
			Name:             "Software Subcontracting",
			NAICSCode:        &naics,
			SetAside:         &setAsideSB,
			ResponseDeadline: &deadline,
			ActiveOnly:       true,
			IsDefault:        true,
			SortOrder:        2,
		},
		{
			UserID:           userID,
			Name:             "SDVOSB Contracts",
			SetAside:         &setAsideSDVOSB,
			ResponseDeadline: &deadline,
			ActiveOnly:       true,
			IsDefault:        true,
			SortOrder:        3,
		},
	}

	for _, f := range defaults {
		if _, err := CreateSavedFilter(db, &f); err != nil {
			return err
		}
	}
	return nil
}
