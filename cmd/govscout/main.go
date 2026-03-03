package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/theognis1002/govscout/internal/alerts"
	"github.com/theognis1002/govscout/internal/db"
	"github.com/theognis1002/govscout/internal/samgov"
	gosync "github.com/theognis1002/govscout/internal/sync"
	"github.com/theognis1002/govscout/internal/web"
)

func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}

func main() {
	loadEnv(".env")
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "sync":
		cmdSync(os.Args[2:])
	case "export":
		cmdExport(os.Args[2:])
	case "useradd":
		cmdUserAdd(os.Args[2:])
	case "passwd":
		cmdPasswd(os.Args[2:])
	case "migrate":
		cmdMigrate(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage: govscout <command> [flags]

Commands:
  serve     Start the web server
  sync      Run sync (incremental + backfill)
  export    Export opportunities to CSV
  useradd   Create a new user
  passwd    Update a user's password
  migrate   Import data from old (Rust) DB

`)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	addr := fs.String("addr", "", "Listen address (default :8080 or PORT env)")
	dev := fs.Bool("dev", false, "Dev mode: reload templates and CSS from disk on each request")
	fs.Parse(args)

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	srv := web.NewServer(database, web.WithDevMode(*dev))
	if err := srv.ListenAndServe(*addr); err != nil {
		log.Fatal(err)
	}
}

func cmdSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	maxCalls := fs.Int("max-calls", 18, "Max API calls for this run")
	dryRun := fs.Bool("dry-run", false, "Preview what would be fetched")
	from := fs.String("from", "", "Backfill target start date (MM/DD/YYYY)")
	fs.Parse(args)

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	apiKey := os.Getenv("SAMGOV_API_KEY")
	client, err := samgov.NewClient(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	if err := gosync.Run(database, client, gosync.Options{
		MaxCalls: *maxCalls,
		DryRun:   *dryRun,
		From:     *from,
	}); err != nil {
		log.Fatal(err)
	}

	if !*dryRun {
		if err := alerts.RunMatcher(database); err != nil {
			log.Printf("alert matcher error: %v", err)
		}
	}
}

func cmdExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	search := fs.String("search", "", "Text search")
	naics := fs.String("naics", "", "NAICS codes (comma-separated)")
	oppType := fs.String("type", "", "Opportunity types (comma-separated)")
	setAside := fs.String("set-aside", "", "Set-aside codes (comma-separated)")
	state := fs.String("state", "", "State code")
	department := fs.String("department", "", "Department (comma-separated)")
	activeOnly := fs.Bool("active-only", false, "Only active opportunities")
	out := fs.String("out", "", "Output file path (default: stdout)")
	fs.Parse(args)

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	filters := db.ListFilters{
		Search:     *search,
		NAICSCode:  *naics,
		OppType:    *oppType,
		SetAside:   *setAside,
		State:      *state,
		Department: *department,
		ActiveOnly: *activeOnly,
	}

	items, err := db.ExportOpportunities(database, filters)
	if err != nil {
		log.Fatal(err)
	}

	var w *os.File
	if *out != "" {
		w, err = os.Create(*out)
		if err != nil {
			log.Fatal(err)
		}
		defer w.Close()
	} else {
		w = os.Stdout
	}

	if err := db.WriteCSV(w, items); err != nil {
		log.Fatal(err)
	}
	if *out != "" {
		fmt.Fprintf(os.Stderr, "exported %d opportunities to %s\n", len(items), *out)
	}
}

func cmdUserAdd(args []string) {
	fs := flag.NewFlagSet("useradd", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	username := fs.String("username", "", "Username (required)")
	password := fs.String("password", "", "Password (required)")
	admin := fs.Bool("admin", false, "Grant admin privileges")
	fs.Parse(args)

	if *username == "" || *password == "" {
		fmt.Fprintf(os.Stderr, "Usage: govscout useradd --username NAME --password PASS [--admin]\n")
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	hash, err := web.HashPassword(*password)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.CreateUser(database, *username, hash, *admin); err != nil {
		log.Fatal(err)
	}
	role := "user"
	if *admin {
		role = "admin"
	}
	fmt.Printf("created %s user: %s\n", role, *username)
}

func cmdPasswd(args []string) {
	fs := flag.NewFlagSet("passwd", flag.ExitOnError)
	dbPath := fs.String("db", "", "SQLite database path")
	username := fs.String("username", "", "Username (required)")
	password := fs.String("password", "", "New password (required)")
	fs.Parse(args)

	if *username == "" || *password == "" {
		fmt.Fprintf(os.Stderr, "Usage: govscout passwd --username NAME --password PASS\n")
		os.Exit(1)
	}

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	hash, err := web.HashPassword(*password)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.UpdatePassword(database, *username, hash); err != nil {
		if err == sql.ErrNoRows {
			log.Fatalf("user %q not found", *username)
		}
		log.Fatal(err)
	}
	fmt.Printf("updated password for user: %s\n", *username)
}

func cmdMigrate(args []string) {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dbPath := fs.String("db", "", "Target SQLite database path")
	oldPath := fs.String("old", "./govscout.db.old", "Old (Rust) database path")
	fs.Parse(args)

	database, err := db.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Attach old DB
	if _, err := database.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS old", *oldPath)); err != nil {
		log.Fatalf("attach old db: %v", err)
	}

	// Check old schema has notice_id
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM old.opportunities").Scan(&count)
	if err != nil {
		log.Fatalf("read old db: %v", err)
	}
	log.Printf("old DB: %d opportunities", count)

	// Migrate opportunities (notice_id -> id, active TEXT -> INTEGER)
	res, err := database.Exec(`INSERT OR IGNORE INTO opportunities (
		id, title, solicitation_number, department, sub_tier, office,
		full_parent_path_name, organization_type, opp_type, base_type,
		posted_date, response_deadline, archive_date, naics_code, classification_code,
		set_aside, set_aside_description, description, ui_link, active, resource_links,
		award_amount, award_date, award_number, awardee_name, awardee_duns, awardee_uei_sam,
		pop_state_code, pop_state_name, pop_city_code, pop_city_name,
		pop_country_code, pop_country_name, pop_zip,
		created_at, modified_at
	)
	SELECT
		notice_id, title, solicitation_number, department, sub_tier, office,
		full_parent_path_name, organization_type, opp_type, base_type,
		posted_date, response_deadline, archive_date, naics_code, classification_code,
		set_aside, set_aside_description, description, ui_link,
		CASE WHEN active = 'Yes' THEN 1 ELSE 0 END,
		resource_links,
		award_amount, award_date, award_number, awardee_name, awardee_duns, awardee_uei_sam,
		pop_state_code, pop_state_name, pop_city_code, pop_city_name,
		pop_country_code, pop_country_name, pop_zip,
		created_at, modified_at
	FROM old.opportunities`)
	if err != nil {
		log.Fatalf("migrate opportunities: %v", err)
	}
	oppRows, _ := res.RowsAffected()
	log.Printf("migrated %d opportunities", oppRows)

	// Migrate contacts
	res, err = database.Exec(`INSERT OR IGNORE INTO contacts (
		id, notice_id, contact_type, full_name, email, phone, title, created_at
	)
	SELECT id, notice_id, contact_type, full_name, email, phone, title, created_at
	FROM old.contacts`)
	if err != nil {
		log.Fatalf("migrate contacts: %v", err)
	}
	contactRows, _ := res.RowsAffected()
	log.Printf("migrated %d contacts", contactRows)

	// Migrate sync_state
	res, err = database.Exec(`INSERT OR REPLACE INTO sync_state (key, value)
		SELECT key, value FROM old.sync_state`)
	if err != nil {
		log.Fatalf("migrate sync_state: %v", err)
	}
	stateRows, _ := res.RowsAffected()
	log.Printf("migrated %d sync_state entries", stateRows)

	// Migrate sync_runs from api_call_log if it exists
	var hasAPILog int
	database.QueryRow("SELECT COUNT(*) FROM old.sqlite_master WHERE type='table' AND name='api_call_log'").Scan(&hasAPILog)
	if hasAPILog > 0 {
		res, err = database.Exec(`INSERT OR IGNORE INTO sync_runs (
			id, started_at, finished_at, context, posted_from, posted_to,
			api_calls, records_fetched, rate_limited, error_message
		)
		SELECT id, timestamp, timestamp, context, posted_from, posted_to,
			api_calls, records_fetched, rate_limited, error_message
		FROM old.api_call_log`)
		if err != nil {
			log.Printf("migrate api_call_log: %v (skipped)", err)
		} else {
			logRows, _ := res.RowsAffected()
			log.Printf("migrated %d api_call_log -> sync_runs", logRows)
		}
	}

	// Also migrate any sync_runs that exist in old DB
	var hasSyncRuns int
	database.QueryRow("SELECT COUNT(*) FROM old.sqlite_master WHERE type='table' AND name='sync_runs'").Scan(&hasSyncRuns)
	if hasSyncRuns > 0 {
		var oldSyncCount int
		database.QueryRow("SELECT COUNT(*) FROM old.sync_runs").Scan(&oldSyncCount)
		if oldSyncCount > 0 {
			res, err = database.Exec(`INSERT OR IGNORE INTO sync_runs (
				id, started_at, finished_at, context, posted_from, posted_to,
				api_calls, records_fetched, rate_limited, error_message
			)
			SELECT id, started_at, finished_at, context, posted_from, posted_to,
				api_calls, records_fetched, rate_limited, error_message
			FROM old.sync_runs`)
			if err != nil {
				log.Printf("migrate sync_runs: %v (skipped)", err)
			} else {
				srRows, _ := res.RowsAffected()
				log.Printf("migrated %d sync_runs", srRows)
			}
		}
	}

	database.Exec("DETACH old")

	// Verify
	var newCount int
	database.QueryRow("SELECT COUNT(*) FROM opportunities").Scan(&newCount)
	var newContacts int
	database.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&newContacts)
	log.Printf("done. new DB: %d opportunities, %d contacts", newCount, newContacts)

	// Remove unused import guard
	_ = sql.ErrNoRows
}
