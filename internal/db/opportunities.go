package db

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OpportunityRow struct {
	ID                  string
	Title               *string
	SolicitationNumber  *string
	Department          *string
	SubTier             *string
	Office              *string
	FullParentPathName  *string
	OrganizationType    *string
	OppType             *string
	BaseType            *string
	PostedDate          *string
	ResponseDeadline    *string
	ArchiveDate         *string
	NAICSCode           *string
	ClassificationCode  *string
	SetAside            *string
	SetAsideDescription *string
	Description         *string
	UILink              *string
	Active              int
	ResourceLinks       *string
	AwardAmount         *string
	AwardDate           *string
	AwardNumber         *string
	AwardeeName         *string
	AwardeeDUNS         *string
	AwardeeUEI          *string
	PopStateCode        *string
	PopStateName        *string
	PopCityCode         *string
	PopCityName         *string
	PopCountryCode      *string
	PopCountryName      *string
	PopZip              *string
	RawJSON             *string
	CreatedAt           string
	ModifiedAt          string
}

type ContactRow struct {
	ID          int64
	NoticeID    string
	ContactType *string
	FullName    *string
	Email       *string
	Phone       *string
	Title       *string
}

type OpportunityListItem struct {
	ID                  string
	Title               *string
	SolicitationNumber  *string
	Department          *string
	SubTier             *string
	Office              *string
	OppType             *string
	BaseType            *string
	PostedDate          *string
	ResponseDeadline    *string
	NAICSCode           *string
	SetAside            *string
	SetAsideDescription *string
	Description         *string
	Active              int
	UILink              *string
	PopStateCode        *string
	PopStateName        *string
}

type ListResult struct {
	Total         int64
	Opportunities []OpportunityListItem
}

type FilterStat struct {
	Value string
	Count int64
}

type Stats struct {
	Total       int64
	NAICSCodes   []FilterStat
	OppTypes    []FilterStat
	SetAsides   []FilterStat
	States      []FilterStat
	Departments []FilterStat
}

type OpportunityDetail struct {
	Opp      OpportunityRow
	Contacts []ContactRow
}

type ListFilters struct {
	Search               string
	NAICSCode            string
	OppType              string
	SetAside             string
	State                string
	Department           string
	DateFrom             string
	DateTo               string
	ResponseDeadline     string
	ResponseDeadlineFrom string
	ResponseDeadlineTo   string
	ActiveOnly           bool
	Limit                int
	Offset               int
}

type QueryBuilder struct {
	clauses []string
	params  []any
}

func (qb *QueryBuilder) addLikeSearch(search string) {
	if search == "" {
		return
	}
	escaped := strings.ReplaceAll(search, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "%", `\%`)
	escaped = strings.ReplaceAll(escaped, "_", `\_`)
	pattern := "%" + escaped + "%"
	qb.clauses = append(qb.clauses,
		`(title LIKE ? ESCAPE '\' OR solicitation_number LIKE ? ESCAPE '\' OR department LIKE ? ESCAPE '\')`)
	qb.params = append(qb.params, pattern, pattern, pattern)
}

func (qb *QueryBuilder) addIn(column string, csv string) {
	vals := splitCSV(csv)
	if len(vals) == 0 {
		return
	}
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		placeholders[i] = "?"
		qb.params = append(qb.params, v)
	}
	qb.clauses = append(qb.clauses, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ",")))
}

func (qb *QueryBuilder) addDateGte(column, value string) {
	if value == "" {
		return
	}
	sortable := mmddyyyyToYyyymmdd(value)
	qb.clauses = append(qb.clauses,
		fmt.Sprintf("substr(%s,7,4)||substr(%s,1,2)||substr(%s,4,2) >= ?", column, column, column))
	qb.params = append(qb.params, sortable)
}

func (qb *QueryBuilder) addDateLte(column, value string) {
	if value == "" {
		return
	}
	sortable := mmddyyyyToYyyymmdd(value)
	qb.clauses = append(qb.clauses,
		fmt.Sprintf("substr(%s,7,4)||substr(%s,1,2)||substr(%s,4,2) <= ?", column, column, column))
	qb.params = append(qb.params, sortable)
}

func (qb *QueryBuilder) addLiteral(clause string) {
	qb.clauses = append(qb.clauses, clause)
}

func (qb *QueryBuilder) whereSQL() string {
	if len(qb.clauses) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(qb.clauses, " AND ")
}

func mmddyyyyToYyyymmdd(date string) string {
	parts := strings.Split(date, "/")
	if len(parts) == 3 {
		return parts[2] + parts[0] + parts[1]
	}
	return date
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func ListOpportunities(db *sql.DB, f ListFilters) (*ListResult, error) {
	var qb QueryBuilder

	qb.addLikeSearch(f.Search)
	qb.addIn("naics_code", f.NAICSCode)
	qb.addIn("opp_type", f.OppType)
	qb.addIn("set_aside", f.SetAside)
	qb.addIn("pop_state_code", f.State)
	qb.addIn("department", f.Department)
	qb.addDateGte("posted_date", f.DateFrom)
	qb.addDateLte("posted_date", f.DateTo)
	qb.addDateGte("response_deadline", f.ResponseDeadlineFrom)
	qb.addDateLte("response_deadline", f.ResponseDeadlineTo)
	if f.ActiveOnly {
		qb.addLiteral("active = 1")
	}

	where := qb.whereSQL()

	// Count
	countSQL := "SELECT COUNT(*) FROM opportunities " + where
	var total int64
	if err := db.QueryRow(countSQL, qb.params...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`SELECT id, title, solicitation_number, department, sub_tier, office,
		opp_type, base_type, posted_date, response_deadline, naics_code,
		set_aside, set_aside_description, description, active, ui_link,
		pop_state_code, pop_state_name
		FROM opportunities %s ORDER BY substr(posted_date,7,4)||substr(posted_date,1,2)||substr(posted_date,4,2) DESC LIMIT ? OFFSET ?`, where)

	params := make([]any, len(qb.params)+2)
	copy(params, qb.params)
	params[len(qb.params)] = limit
	params[len(qb.params)+1] = offset
	rows, err := db.Query(query, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var items []OpportunityListItem
	for rows.Next() {
		var o OpportunityListItem
		if err := rows.Scan(
			&o.ID, &o.Title, &o.SolicitationNumber, &o.Department, &o.SubTier, &o.Office,
			&o.OppType, &o.BaseType, &o.PostedDate, &o.ResponseDeadline, &o.NAICSCode,
			&o.SetAside, &o.SetAsideDescription, &o.Description, &o.Active, &o.UILink,
			&o.PopStateCode, &o.PopStateName,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		items = append(items, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	return &ListResult{Total: total, Opportunities: items}, nil
}

func ExportOpportunities(database *sql.DB, f ListFilters) ([]OpportunityListItem, error) {
	var qb QueryBuilder

	qb.addLikeSearch(f.Search)
	qb.addIn("naics_code", f.NAICSCode)
	qb.addIn("opp_type", f.OppType)
	qb.addIn("set_aside", f.SetAside)
	qb.addIn("pop_state_code", f.State)
	qb.addIn("department", f.Department)
	qb.addDateGte("posted_date", f.DateFrom)
	qb.addDateLte("posted_date", f.DateTo)
	qb.addDateGte("response_deadline", f.ResponseDeadlineFrom)
	qb.addDateLte("response_deadline", f.ResponseDeadlineTo)
	if f.ActiveOnly {
		qb.addLiteral("active = 1")
	}

	where := qb.whereSQL()

	query := fmt.Sprintf(`SELECT id, title, solicitation_number, department, sub_tier, office,
		opp_type, base_type, posted_date, response_deadline, naics_code,
		set_aside, set_aside_description, description, active, ui_link,
		pop_state_code, pop_state_name
		FROM opportunities %s ORDER BY substr(posted_date,7,4)||substr(posted_date,1,2)||substr(posted_date,4,2) DESC`, where)

	rows, err := database.Query(query, qb.params...)
	if err != nil {
		return nil, fmt.Errorf("export query: %w", err)
	}
	defer rows.Close()

	var items []OpportunityListItem
	for rows.Next() {
		var o OpportunityListItem
		if err := rows.Scan(
			&o.ID, &o.Title, &o.SolicitationNumber, &o.Department, &o.SubTier, &o.Office,
			&o.OppType, &o.BaseType, &o.PostedDate, &o.ResponseDeadline, &o.NAICSCode,
			&o.SetAside, &o.SetAsideDescription, &o.Description, &o.Active, &o.UILink,
			&o.PopStateCode, &o.PopStateName,
		); err != nil {
			return nil, fmt.Errorf("export scan: %w", err)
		}
		items = append(items, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("export rows: %w", err)
	}
	return items, nil
}

func WriteCSV(w io.Writer, items []OpportunityListItem) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"ID", "Title", "Solicitation Number", "Department", "Sub Tier", "Office",
		"Type", "Posted Date", "Response Deadline", "NAICS Code", "Set-Aside",
		"State", "Active", "SAM.gov Link", "Description"}
	if err := cw.Write(header); err != nil {
		return err
	}

	deref := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
	}

	for _, o := range items {
		active := "No"
		if o.Active == 1 {
			active = "Yes"
		}
		row := []string{
			o.ID, deref(o.Title), deref(o.SolicitationNumber), deref(o.Department),
			deref(o.SubTier), deref(o.Office), deref(o.OppType), deref(o.PostedDate),
			deref(o.ResponseDeadline), deref(o.NAICSCode), deref(o.SetAside),
			deref(o.PopStateCode), active, deref(o.UILink), deref(o.Description),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return cw.Error()
}

func GetOpportunity(database *sql.DB, id string) (*OpportunityDetail, error) {
	row := database.QueryRow(`SELECT id, title, solicitation_number, department, sub_tier, office,
		full_parent_path_name, organization_type, opp_type, base_type,
		posted_date, response_deadline, archive_date, naics_code, classification_code,
		set_aside, set_aside_description, description, ui_link, active, resource_links,
		award_amount, award_date, award_number, awardee_name, awardee_duns, awardee_uei_sam,
		pop_state_code, pop_state_name, pop_city_code, pop_city_name,
		pop_country_code, pop_country_name, pop_zip, raw_json,
		created_at, modified_at
		FROM opportunities WHERE id = ?`, id)

	var o OpportunityRow
	err := row.Scan(
		&o.ID, &o.Title, &o.SolicitationNumber, &o.Department, &o.SubTier, &o.Office,
		&o.FullParentPathName, &o.OrganizationType, &o.OppType, &o.BaseType,
		&o.PostedDate, &o.ResponseDeadline, &o.ArchiveDate, &o.NAICSCode, &o.ClassificationCode,
		&o.SetAside, &o.SetAsideDescription, &o.Description, &o.UILink, &o.Active, &o.ResourceLinks,
		&o.AwardAmount, &o.AwardDate, &o.AwardNumber, &o.AwardeeName, &o.AwardeeDUNS, &o.AwardeeUEI,
		&o.PopStateCode, &o.PopStateName, &o.PopCityCode, &o.PopCityName,
		&o.PopCountryCode, &o.PopCountryName, &o.PopZip, &o.RawJSON,
		&o.CreatedAt, &o.ModifiedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan opportunity: %w", err)
	}

	contactRows, err := database.Query(
		`SELECT id, notice_id, contact_type, full_name, email, phone, title
		FROM contacts WHERE notice_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer contactRows.Close()

	var contacts []ContactRow
	for contactRows.Next() {
		var c ContactRow
		if err := contactRows.Scan(&c.ID, &c.NoticeID, &c.ContactType, &c.FullName, &c.Email, &c.Phone, &c.Title); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	if err := contactRows.Err(); err != nil {
		return nil, fmt.Errorf("contact rows: %w", err)
	}

	return &OpportunityDetail{Opp: o, Contacts: contacts}, nil
}

func GetFilterStats(database *sql.DB) (*Stats, error) {
	var s Stats
	if err := database.QueryRow("SELECT COUNT(*) FROM opportunities").Scan(&s.Total); err != nil {
		return nil, err
	}

	statQueries := []struct {
		query string
		dest  *[]FilterStat
	}{
		{"SELECT naics_code, COUNT(*) FROM opportunities WHERE naics_code IS NOT NULL AND naics_code != '' GROUP BY naics_code ORDER BY COUNT(*) DESC", &s.NAICSCodes},
		{"SELECT opp_type, COUNT(*) FROM opportunities WHERE opp_type IS NOT NULL AND opp_type != '' GROUP BY opp_type ORDER BY COUNT(*) DESC", &s.OppTypes},
		{"SELECT set_aside, COUNT(*) FROM opportunities WHERE set_aside IS NOT NULL AND set_aside != '' GROUP BY set_aside ORDER BY COUNT(*) DESC", &s.SetAsides},
		{"SELECT pop_state_code, COUNT(*) FROM opportunities WHERE pop_state_code IS NOT NULL AND pop_state_code != '' GROUP BY pop_state_code ORDER BY COUNT(*) DESC", &s.States},
		{"SELECT department, COUNT(*) FROM opportunities WHERE department IS NOT NULL AND department != '' GROUP BY department ORDER BY COUNT(*) DESC", &s.Departments},
	}

	for _, sq := range statQueries {
		rows, err := database.Query(sq.query)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var fs FilterStat
			if err := rows.Scan(&fs.Value, &fs.Count); err != nil {
				rows.Close()
				return nil, err
			}
			*sq.dest = append(*sq.dest, fs)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		rows.Close()
	}

	return &s, nil
}

func UpsertOpportunity(tx *sql.Tx, id string, title, solNum, dept, subTier, office,
	fullParent, orgType, oppType, baseType, postedDate, responseDeadline, archiveDate,
	naicsCode, classCode, setAside, setAsideDesc, description, uiLink *string,
	active int, resourceLinks *string,
	awardAmount, awardDate, awardNumber, awardeeName, awardeeDUNS, awardeeUEI,
	popStateCode, popStateName, popCityCode, popCityName,
	popCountryCode, popCountryName, popZip, rawJSON *string) error {

	_, err := tx.Exec(`INSERT INTO opportunities (
		id, title, solicitation_number, department, sub_tier, office,
		full_parent_path_name, organization_type, opp_type, base_type,
		posted_date, response_deadline, archive_date, naics_code, classification_code,
		set_aside, set_aside_description, description, ui_link, active, resource_links,
		award_amount, award_date, award_number, awardee_name, awardee_duns, awardee_uei_sam,
		pop_state_code, pop_state_name, pop_city_code, pop_city_name,
		pop_country_code, pop_country_name, pop_zip, raw_json
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET
		title=excluded.title, solicitation_number=excluded.solicitation_number,
		department=excluded.department, sub_tier=excluded.sub_tier, office=excluded.office,
		full_parent_path_name=excluded.full_parent_path_name,
		organization_type=excluded.organization_type, opp_type=excluded.opp_type,
		base_type=excluded.base_type, posted_date=excluded.posted_date,
		response_deadline=excluded.response_deadline, archive_date=excluded.archive_date,
		naics_code=excluded.naics_code, classification_code=excluded.classification_code,
		set_aside=excluded.set_aside, set_aside_description=excluded.set_aside_description,
		description=excluded.description, ui_link=excluded.ui_link, active=excluded.active,
		resource_links=excluded.resource_links,
		award_amount=excluded.award_amount, award_date=excluded.award_date,
		award_number=excluded.award_number, awardee_name=excluded.awardee_name,
		awardee_duns=excluded.awardee_duns, awardee_uei_sam=excluded.awardee_uei_sam,
		pop_state_code=excluded.pop_state_code, pop_state_name=excluded.pop_state_name,
		pop_city_code=excluded.pop_city_code, pop_city_name=excluded.pop_city_name,
		pop_country_code=excluded.pop_country_code, pop_country_name=excluded.pop_country_name,
		pop_zip=excluded.pop_zip, raw_json=excluded.raw_json,
		modified_at=datetime('now')`,
		id, title, solNum, dept, subTier, office,
		fullParent, orgType, oppType, baseType,
		postedDate, responseDeadline, archiveDate, naicsCode, classCode,
		setAside, setAsideDesc, description, uiLink, active, resourceLinks,
		awardAmount, awardDate, awardNumber, awardeeName, awardeeDUNS, awardeeUEI,
		popStateCode, popStateName, popCityCode, popCityName,
		popCountryCode, popCountryName, popZip, rawJSON,
	)
	return err
}

func ReplaceContacts(tx *sql.Tx, noticeID string, contacts []ContactRow) error {
	if _, err := tx.Exec("DELETE FROM contacts WHERE notice_id = ?", noticeID); err != nil {
		return err
	}
	for _, c := range contacts {
		if _, err := tx.Exec(
			"INSERT INTO contacts (notice_id, contact_type, full_name, email, phone, title) VALUES (?,?,?,?,?,?)",
			noticeID, c.ContactType, c.FullName, c.Email, c.Phone, c.Title,
		); err != nil {
			return err
		}
	}
	return nil
}

func UpsertOpportunityFromAPI(db *sql.DB, opp map[string]any) error {
	noticeID, _ := opp["noticeId"].(string)
	if noticeID == "" {
		return nil
	}

	str := func(key string) *string {
		if v, ok := opp[key].(string); ok {
			return &v
		}
		return nil
	}

	activeInt := 0
	if a, ok := opp["active"].(string); ok && a == "Yes" {
		activeInt = 1
	}

	var resourceLinksJSON *string
	if rl, ok := opp["resourceLinks"].([]any); ok {
		b, _ := json.Marshal(rl)
		s := string(b)
		resourceLinksJSON = &s
	}

	var awardAmount, awardDate, awardNumber, awardeeName, awardeeDUNS, awardeeUEI *string
	if award, ok := opp["award"].(map[string]any); ok {
		if v, ok := award["amount"].(string); ok {
			awardAmount = &v
		}
		if v, ok := award["date"].(string); ok {
			awardDate = &v
		}
		if v, ok := award["number"].(string); ok {
			awardNumber = &v
		}
		if awardee, ok := award["awardee"].(map[string]any); ok {
			if v, ok := awardee["name"].(string); ok {
				awardeeName = &v
			}
			if v, ok := awardee["duns"].(string); ok {
				awardeeDUNS = &v
			}
			if v, ok := awardee["ueiSAM"].(string); ok {
				awardeeUEI = &v
			}
		}
	}

	var popStateCode, popStateName, popCityCode, popCityName, popCountryCode, popCountryName, popZip *string
	if pop, ok := opp["placeOfPerformance"].(map[string]any); ok {
		if st, ok := pop["state"].(map[string]any); ok {
			if v, ok := st["code"].(string); ok {
				popStateCode = &v
			}
			if v, ok := st["name"].(string); ok {
				popStateName = &v
			}
		}
		if ct, ok := pop["city"].(map[string]any); ok {
			if v, ok := ct["code"].(string); ok {
				popCityCode = &v
			}
			if v, ok := ct["name"].(string); ok {
				popCityName = &v
			}
		}
		if co, ok := pop["country"].(map[string]any); ok {
			if v, ok := co["code"].(string); ok {
				popCountryCode = &v
			}
			if v, ok := co["name"].(string); ok {
				popCountryName = &v
			}
		}
		if v, ok := pop["zip"].(string); ok {
			popZip = &v
		}
	}

	rawBytes, _ := json.Marshal(opp)
	rawStr := string(rawBytes)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// SAM.gov v2: department is deprecated; extract from fullParentPathName
	dept := str("department")
	if dept == nil {
		if fp := str("fullParentPathName"); fp != nil {
			if idx := strings.Index(*fp, "."); idx > 0 {
				d := (*fp)[:idx]
				dept = &d
			} else {
				dept = fp
			}
		}
	}

	// SAM.gov v2: setAside → typeOfSetAside, setAsideDescription → typeOfSetAsideDescription
	setAside := str("typeOfSetAside")
	if setAside == nil {
		setAside = str("setAside")
	}
	setAsideDesc := str("typeOfSetAsideDescription")
	if setAsideDesc == nil {
		setAsideDesc = str("setAsideDescription")
	}

	if err := UpsertOpportunity(tx, noticeID,
		str("title"), str("solicitationNumber"), dept, str("subTier"), str("office"),
		str("fullParentPathName"), str("organizationType"), str("type"), str("baseType"),
		str("postedDate"), str("responseDeadline"), str("archiveDate"),
		str("naicsCode"), str("classificationCode"), setAside, setAsideDesc,
		str("description"), str("uiLink"), activeInt, resourceLinksJSON,
		awardAmount, awardDate, awardNumber, awardeeName, awardeeDUNS, awardeeUEI,
		popStateCode, popStateName, popCityCode, popCityName,
		popCountryCode, popCountryName, popZip, &rawStr,
	); err != nil {
		return fmt.Errorf("upsert opportunity %s: %w", noticeID, err)
	}

	// Replace contacts
	var contacts []ContactRow
	if pocs, ok := opp["pointOfContact"].([]any); ok {
		for _, poc := range pocs {
			if m, ok := poc.(map[string]any); ok {
				var c ContactRow
				c.NoticeID = noticeID
				if v, ok := m["type"].(string); ok {
					c.ContactType = &v
				}
				if v, ok := m["fullName"].(string); ok {
					c.FullName = &v
				}
				if v, ok := m["email"].(string); ok {
					c.Email = &v
				}
				if v, ok := m["phone"].(string); ok {
					c.Phone = &v
				}
				if v, ok := m["title"].(string); ok {
					c.Title = &v
				}
				contacts = append(contacts, c)
			}
		}
	}
	if err := ReplaceContacts(tx, noticeID, contacts); err != nil {
		return fmt.Errorf("replace contacts %s: %w", noticeID, err)
	}

	return tx.Commit()
}
