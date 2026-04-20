package samgov

type APIResponse struct {
	TotalRecords      *int64           `json:"totalRecords"`
	OpportunitiesData []map[string]any `json:"opportunitiesData"`
}

type SearchParams struct {
	Limit      int
	Offset     int
	PostedFrom string
	PostedTo   string
	Title      string
	Type       string
	NAICS      string
	State      string
	SetAside   string
	NoticeID   string
}
