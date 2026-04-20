package web

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/theognis1002/govscout/internal/alerts"
	"github.com/theognis1002/govscout/internal/db"
	"github.com/theognis1002/govscout/internal/samgov"
	gosync "github.com/theognis1002/govscout/internal/sync"
)

// Auth handlers

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.getSession(r) != nil {
		http.Redirect(w, r, "/opportunities", http.StatusFound)
		return
	}
	s.render(w, r, "login.html", nil)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := db.GetUserByUsername(s.db, username)
	if err != nil || user == nil || !CheckPassword(user.PasswordHash, password) {
		s.render(w, r, "login.html", map[string]any{"Error": "Invalid username or password"})
		return
	}

	s.setSession(w, user)
	setFlash(w, "success", "Signed in")
	http.Redirect(w, r, "/opportunities", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// Opportunities handlers

func (s *Server) handleOpportunities(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)

	// Seed default filters on first visit
	db.SeedDefaultFilters(s.db, user.ID)
	savedFilters, _ := db.ListSavedFilters(s.db, user.ID)

	// Apply saved filter if filter_id is present
	var activeFilterID int64
	if fid := r.URL.Query().Get("filter_id"); fid != "" {
		if id, err := strconv.ParseInt(fid, 10, 64); err == nil {
			if sf, err := db.GetSavedFilter(s.db, id); err == nil && sf != nil && sf.UserID == user.ID {
				activeFilterID = sf.ID
				q := r.URL.Query()
				if sf.SearchQuery != nil {
					q.Set("search", *sf.SearchQuery)
				}
				if sf.NAICSCode != nil {
					q.Set("naics_code", *sf.NAICSCode)
				}
				if sf.OppType != nil {
					q.Set("opp_type", *sf.OppType)
				}
				if sf.SetAside != nil {
					q.Set("set_aside", *sf.SetAside)
				}
				if sf.State != nil {
					q.Set("state", *sf.State)
				}
				if sf.Department != nil {
					q.Set("department", *sf.Department)
				}
				if sf.ActiveOnly {
					q.Set("active_only", "on")
				}
				if sf.ResponseDeadline != nil {
					q.Set("response_deadline", *sf.ResponseDeadline)
				}
				r.URL.RawQuery = q.Encode()
			}
		}
	}

	filters := parseFilters(r)
	result, err := db.ListOpportunities(s.db, filters)
	if err != nil {
		log.Printf("list opportunities: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	stats, _ := db.GetFilterStats(s.db)

	data := map[string]any{
		"Opportunities":  result.Opportunities,
		"Total":          result.Total,
		"Filters":        filters,
		"Stats":          stats,
		"PageCount":      pageCount(result.Total, filters.Limit),
		"CurrentPage":    currentPage(filters.Offset, filters.Limit),
		"SavedFilters":   savedFilters,
		"ActiveFilterID": activeFilterID,
	}
	s.render(w, r, "opportunities.html", data)
}

func (s *Server) handleOpportunitiesPartial(w http.ResponseWriter, r *http.Request) {
	filters := parseFilters(r)
	result, err := db.ListOpportunities(s.db, filters)
	if err != nil {
		log.Printf("list opportunities partial: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	data := map[string]any{
		"Opportunities": result.Opportunities,
		"Total":         result.Total,
		"Filters":       filters,
		"PageCount":     pageCount(result.Total, filters.Limit),
		"CurrentPage":   currentPage(filters.Offset, filters.Limit),
	}
	renderTemplate(w, s.templates(), "results.html", data)
}

func (s *Server) handleOpportunityDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := db.GetOpportunity(s.db, id)
	if err != nil {
		log.Printf("get opportunity %s: %v", id, err)
		http.Error(w, "Internal server error", 500)
		return
	}
	if detail == nil {
		http.NotFound(w, r)
		return
	}
	s.render(w, r, "opportunity.html", map[string]any{"Detail": detail})
}

func (s *Server) handleOpportunitiesExport(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)

	// Apply saved filter if filter_id is present
	if fid := r.URL.Query().Get("filter_id"); fid != "" {
		if id, err := strconv.ParseInt(fid, 10, 64); err == nil {
			if sf, err := db.GetSavedFilter(s.db, id); err == nil && sf != nil && sf.UserID == user.ID {
				q := r.URL.Query()
				if sf.SearchQuery != nil {
					q.Set("search", *sf.SearchQuery)
				}
				if sf.NAICSCode != nil {
					q.Set("naics_code", *sf.NAICSCode)
				}
				if sf.OppType != nil {
					q.Set("opp_type", *sf.OppType)
				}
				if sf.SetAside != nil {
					q.Set("set_aside", *sf.SetAside)
				}
				if sf.State != nil {
					q.Set("state", *sf.State)
				}
				if sf.Department != nil {
					q.Set("department", *sf.Department)
				}
				if sf.ActiveOnly {
					q.Set("active_only", "on")
				}
				if sf.ResponseDeadline != nil {
					q.Set("response_deadline", *sf.ResponseDeadline)
				}
				r.URL.RawQuery = q.Encode()
			}
		}
	}

	filters := parseFilters(r)
	items, err := db.ExportOpportunities(s.db, filters)
	if err != nil {
		log.Printf("export opportunities: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="opportunities.csv"`)
	if err := db.WriteCSV(w, items); err != nil {
		log.Printf("write csv: %v", err)
	}
}

// Alert handlers

func (s *Server) handleAlertsList(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	db.SeedDefaultSearches(s.db, user.ID)
	searches, err := db.ListSavedSearches(s.db, user.ID)
	if err != nil {
		log.Printf("list saved searches: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	alertsList, _, err := db.ListAlertsForUser(s.db, user.ID, 50, 0)
	if err != nil {
		log.Printf("list alerts for user: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	s.render(w, r, "alerts_list.html", map[string]any{
		"Searches": searches,
		"Alerts":   alertsList,
	})
}

func (s *Server) handleAlertForm(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetFilterStats(s.db)
	if err != nil {
		log.Printf("get filter stats: %v", err)
	}
	s.render(w, r, "alerts_form.html", map[string]any{"Stats": stats})
}

func (s *Server) handleAlertCreate(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	search := &db.SavedSearchRow{
		UserID:     user.ID,
		Name:       r.FormValue("name"),
		ActiveOnly: r.FormValue("active_only") == "on",
		MatchAll:   r.FormValue("match_all") == "on",
		Enabled:    r.FormValue("enabled") == "on",
	}
	setOptional(&search.SearchQuery, r.FormValue("search_query"))
	setOptional(&search.NAICSCode, formMultiValue(r, "naics_code"))
	setOptional(&search.OppType, formMultiValue(r, "opp_type"))
	setOptional(&search.SetAside, formMultiValue(r, "set_aside"))
	setOptional(&search.State, formMultiValue(r, "state"))
	setOptional(&search.Department, formMultiValue(r, "department"))
	setOptional(&search.IncludeKeywords, r.FormValue("include_keywords"))
	setOptional(&search.ExcludeKeywords, r.FormValue("exclude_keywords"))
	setOptional(&search.NotifyEmail, r.FormValue("notify_email"))
	setOptional(&search.ResponseDeadline, r.FormValue("response_deadline"))

	id, err := db.CreateSavedSearch(s.db, search)
	if err != nil {
		log.Printf("create saved search: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	setFlash(w, "success", "Alert created")
	http.Redirect(w, r, fmt.Sprintf("/alerts/%d", id), http.StatusFound)
}

func (s *Server) handleAlertDetail(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	search, err := db.GetSavedSearch(s.db, id)
	if err != nil || search == nil {
		http.NotFound(w, r)
		return
	}

	user := getUser(r)
	if search.UserID != user.ID && !user.IsAdmin {
		http.Error(w, "Forbidden", 403)
		return
	}

	alertsList, total, err := db.ListAlertsForSearch(s.db, id, 50, 0)
	if err != nil {
		log.Printf("list alerts for search: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	stats, err := db.GetFilterStats(s.db)
	if err != nil {
		log.Printf("get filter stats: %v", err)
	}
	s.render(w, r, "alerts_detail.html", map[string]any{
		"Search":     search,
		"Alerts":     alertsList,
		"AlertTotal": total,
		"Stats":      stats,
	})
}

func (s *Server) handleAlertUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	search, err := db.GetSavedSearch(s.db, id)
	if err != nil || search == nil {
		http.NotFound(w, r)
		return
	}

	user := getUser(r)
	if search.UserID != user.ID && !user.IsAdmin {
		http.Error(w, "Forbidden", 403)
		return
	}

	search.Name = r.FormValue("name")
	search.ActiveOnly = r.FormValue("active_only") == "on"
	search.MatchAll = r.FormValue("match_all") == "on"
	search.Enabled = r.FormValue("enabled") == "on"
	setOptional(&search.SearchQuery, r.FormValue("search_query"))
	setOptional(&search.NAICSCode, formMultiValue(r, "naics_code"))
	setOptional(&search.OppType, formMultiValue(r, "opp_type"))
	setOptional(&search.SetAside, formMultiValue(r, "set_aside"))
	setOptional(&search.State, formMultiValue(r, "state"))
	setOptional(&search.Department, formMultiValue(r, "department"))
	setOptional(&search.IncludeKeywords, r.FormValue("include_keywords"))
	setOptional(&search.ExcludeKeywords, r.FormValue("exclude_keywords"))
	setOptional(&search.NotifyEmail, r.FormValue("notify_email"))
	setOptional(&search.ResponseDeadline, r.FormValue("response_deadline"))

	if err := db.UpdateSavedSearch(s.db, search); err != nil {
		log.Printf("update saved search: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	setFlash(w, "success", "Alert updated")
	http.Redirect(w, r, fmt.Sprintf("/alerts/%d", id), http.StatusFound)
}

func (s *Server) handleAlertToggle(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	search, err := db.GetSavedSearch(s.db, id)
	if err != nil || search == nil {
		http.NotFound(w, r)
		return
	}

	user := getUser(r)
	if search.UserID != user.ID && !user.IsAdmin {
		http.Error(w, "Forbidden", 403)
		return
	}

	search.Enabled = !search.Enabled
	if err := db.UpdateSavedSearch(s.db, search); err != nil {
		log.Printf("toggle saved search %d: %v", id, err)
		http.Error(w, "Internal server error", 500)
		return
	}
	setFlash(w, "success", "Alert toggled")
	http.Redirect(w, r, fmt.Sprintf("/alerts/%d", id), http.StatusFound)
}

func (s *Server) handleAlertPreview(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	search, err := db.GetSavedSearch(s.db, id)
	if err != nil || search == nil {
		http.NotFound(w, r)
		return
	}

	user := getUser(r)
	if search.UserID != user.ID && !user.IsAdmin {
		http.Error(w, "Forbidden", 403)
		return
	}

	matches, err := alerts.PreviewMatches(s.db, *search, 20)
	if err != nil {
		log.Printf("preview matches: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	stats, err := db.GetFilterStats(s.db)
	if err != nil {
		log.Printf("get filter stats: %v", err)
	}
	s.render(w, r, "alerts_detail.html", map[string]any{
		"Search":  search,
		"Preview": matches,
		"Stats":   stats,
	})
}

// Filter handlers

func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	filters, err := db.ListSavedFilters(s.db, user.ID)
	if err != nil {
		log.Printf("list saved filters: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	stats, _ := db.GetFilterStats(s.db)
	s.render(w, r, "filters_list.html", map[string]any{"Filters": filters, "Stats": stats})
}

func (s *Server) handleFilterCreate(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	f := &db.SavedFilterRow{
		UserID:     user.ID,
		Name:       r.FormValue("name"),
		ActiveOnly: r.FormValue("active_only") == "on",
	}
	setOptional(&f.SearchQuery, r.FormValue("search_query"))
	setOptional(&f.NAICSCode, formMultiValue(r, "naics_code"))
	setOptional(&f.OppType, formMultiValue(r, "opp_type"))
	setOptional(&f.SetAside, formMultiValue(r, "set_aside"))
	setOptional(&f.State, r.FormValue("state"))
	setOptional(&f.Department, formMultiValue(r, "department"))
	setOptional(&f.ResponseDeadline, r.FormValue("response_deadline"))

	if _, err := db.CreateSavedFilter(s.db, f); err != nil {
		log.Printf("create saved filter: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	setFlash(w, "success", "Filter created")
	http.Redirect(w, r, "/filters", http.StatusFound)
}

func (s *Server) handleFilterEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	f, err := db.GetSavedFilter(s.db, id)
	if err != nil || f == nil {
		http.NotFound(w, r)
		return
	}
	user := getUser(r)
	if f.UserID != user.ID {
		http.Error(w, "Forbidden", 403)
		return
	}
	stats, _ := db.GetFilterStats(s.db)
	s.render(w, r, "filters_edit.html", map[string]any{"Filter": f, "Stats": stats})
}

func (s *Server) handleFilterUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	f, err := db.GetSavedFilter(s.db, id)
	if err != nil || f == nil {
		http.NotFound(w, r)
		return
	}
	user := getUser(r)
	if f.UserID != user.ID {
		http.Error(w, "Forbidden", 403)
		return
	}

	f.Name = r.FormValue("name")
	f.ActiveOnly = r.FormValue("active_only") == "on"
	setOptional(&f.SearchQuery, r.FormValue("search_query"))
	setOptional(&f.NAICSCode, formMultiValue(r, "naics_code"))
	setOptional(&f.OppType, formMultiValue(r, "opp_type"))
	setOptional(&f.SetAside, formMultiValue(r, "set_aside"))
	setOptional(&f.State, r.FormValue("state"))
	setOptional(&f.Department, formMultiValue(r, "department"))
	setOptional(&f.ResponseDeadline, r.FormValue("response_deadline"))

	if err := db.UpdateSavedFilter(s.db, f); err != nil {
		log.Printf("update saved filter: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	setFlash(w, "success", "Filter updated")
	http.Redirect(w, r, "/filters", http.StatusFound)
}

func (s *Server) handleFilterDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}
	user := getUser(r)
	db.DeleteSavedFilter(s.db, id, user.ID)
	setFlash(w, "success", "Filter deleted")
	http.Redirect(w, r, "/filters", http.StatusFound)
}

// Admin handlers

func (s *Server) handleAdminSync(w http.ResponseWriter, r *http.Request) {
	apiKey := os.Getenv("SAMGOV_API_KEY")
	client, err := samgov.NewClient(apiKey)
	if err != nil {
		setFlash(w, "error", fmt.Sprintf("Cannot start sync: %v", err))
		http.Redirect(w, r, "/admin/sync-runs", http.StatusFound)
		return
	}

	if !s.syncing.CompareAndSwap(false, true) {
		setFlash(w, "error", "Sync already in progress")
		http.Redirect(w, r, "/admin/sync-runs", http.StatusFound)
		return
	}

	maxCalls := 18
	if mc := r.FormValue("max_calls"); mc != "" {
		if n, err := strconv.Atoi(mc); err == nil && n > 0 {
			maxCalls = n
		}
	}

	go func() {
		defer s.syncing.Store(false)
		if err := gosync.Run(s.db, client, gosync.Options{MaxCalls: maxCalls}); err != nil {
			log.Printf("sync error: %v", err)
			return
		}
		if err := alerts.RunMatcher(s.db); err != nil {
			log.Printf("alert matcher error: %v", err)
		}
	}()
	setFlash(w, "success", "Sync started")
	http.Redirect(w, r, "/admin/sync-runs", http.StatusFound)
}

func (s *Server) handleAdminSyncRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := db.ListSyncRuns(s.db, 50)
	if err != nil {
		log.Printf("list sync runs: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	s.render(w, r, "admin_sync.html", map[string]any{"Runs": runs})
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := db.ListUsers(s.db)
	if err != nil {
		log.Printf("list users: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}
	s.render(w, r, "admin_users.html", map[string]any{"Users": users})
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	isAdmin := r.FormValue("is_admin") == "on"

	if username == "" || password == "" {
		users, _ := db.ListUsers(s.db)
		s.render(w, r, "admin_users.html", map[string]any{
			"Users": users,
			"Error": "Username and password are required",
		})
		return
	}

	hash, err := HashPassword(password)
	if err != nil {
		log.Printf("hash password: %v", err)
		http.Error(w, "Internal server error", 500)
		return
	}

	if err := db.CreateUser(s.db, username, hash, isAdmin); err != nil {
		users, _ := db.ListUsers(s.db)
		s.render(w, r, "admin_users.html", map[string]any{
			"Users": users,
			"Error": fmt.Sprintf("Failed to create user: %v", err),
		})
		return
	}
	setFlash(w, "success", "User created")
	http.Redirect(w, r, "/admin/users", http.StatusFound)
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	// Prevent self-deletion
	user := getUser(r)
	if user.ID == id {
		setFlash(w, "error", "Cannot delete yourself")
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	db.DeleteUser(s.db, id)
	setFlash(w, "success", "User deleted")
	http.Redirect(w, r, "/admin/users", http.StatusFound)
}

// Flash messages

func setFlash(w http.ResponseWriter, kind, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    kind + ":" + msg,
		Path:     "/",
		MaxAge:   10,
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
}

// Helpers

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["User"] = getUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := renderTemplate(w, s.templates(), name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func parseFilters(r *http.Request) db.ListFilters {
	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}
	page := r.URL.Query().Get("page")
	if page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			offset = (p - 1) * limit
		}
	}

	f := db.ListFilters{
		Search:     r.URL.Query().Get("search"),
		NAICSCode:  formMultiValue(r, "naics_code"),
		OppType:    formMultiValue(r, "opp_type"),
		SetAside:   formMultiValue(r, "set_aside"),
		State:      r.URL.Query().Get("state"),
		Department: formMultiValue(r, "department"),
		ActiveOnly: r.URL.Query().Get("active_only") == "on" || r.URL.Query().Get("active_only") == "true",
		Limit:      limit,
		Offset:     offset,
	}

	// Response deadline shortcuts
	if deadline := r.URL.Query().Get("response_deadline"); deadline != "" {
		f.ResponseDeadline = deadline
		now := time.Now()
		f.ResponseDeadlineFrom = now.Format("01/02/2006")
		switch deadline {
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

func parseID(r *http.Request) (int64, error) {
	idStr := chi.URLParam(r, "id")
	return strconv.ParseInt(idStr, 10, 64)
}

func formMultiValue(r *http.Request, key string) string {
	r.ParseForm()
	vals := r.Form[key]
	if len(vals) == 0 {
		return ""
	}
	return strings.Join(vals, ",")
}

func setOptional(ptr **string, val string) {
	if val != "" {
		*ptr = &val
	} else {
		*ptr = nil
	}
}

func pageCount(total int64, limit int) int {
	if limit <= 0 {
		return 1
	}
	p := int(total) / limit
	if int(total)%limit != 0 {
		p++
	}
	return p
}

func currentPage(offset, limit int) int {
	if limit <= 0 {
		return 1
	}
	return offset/limit + 1
}
