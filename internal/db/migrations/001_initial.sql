-- GovScout schema

CREATE TABLE IF NOT EXISTS opportunities (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT,
    solicitation_number TEXT,
    department TEXT,
    sub_tier TEXT,
    office TEXT,
    full_parent_path_name TEXT,
    organization_type TEXT,
    opp_type TEXT,
    base_type TEXT,
    posted_date TEXT,
    response_deadline TEXT,
    archive_date TEXT,
    naics_code TEXT,
    classification_code TEXT,
    set_aside TEXT,
    set_aside_description TEXT,
    description TEXT,
    ui_link TEXT,
    active INTEGER NOT NULL DEFAULT 0,
    resource_links TEXT,
    award_amount TEXT,
    award_date TEXT,
    award_number TEXT,
    awardee_name TEXT,
    awardee_duns TEXT,
    awardee_uei_sam TEXT,
    pop_state_code TEXT,
    pop_state_name TEXT,
    pop_city_code TEXT,
    pop_city_name TEXT,
    pop_country_code TEXT,
    pop_country_name TEXT,
    pop_zip TEXT,
    raw_json TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_opp_posted_date ON opportunities(posted_date);
CREATE INDEX IF NOT EXISTS idx_opp_active ON opportunities(active);
CREATE INDEX IF NOT EXISTS idx_opp_naics_code ON opportunities(naics_code);
CREATE INDEX IF NOT EXISTS idx_opp_opp_type ON opportunities(opp_type);
CREATE INDEX IF NOT EXISTS idx_opp_set_aside ON opportunities(set_aside);
CREATE INDEX IF NOT EXISTS idx_opp_pop_state ON opportunities(pop_state_code);
CREATE INDEX IF NOT EXISTS idx_opp_department ON opportunities(department);
CREATE INDEX IF NOT EXISTS idx_opp_response_deadline ON opportunities(response_deadline);
CREATE INDEX IF NOT EXISTS idx_opp_naics_type ON opportunities(naics_code, opp_type);

CREATE TABLE IF NOT EXISTS contacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    notice_id TEXT NOT NULL REFERENCES opportunities(id) ON DELETE CASCADE,
    contact_type TEXT,
    full_name TEXT,
    email TEXT,
    phone TEXT,
    title TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_contacts_notice ON contacts(notice_id);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    is_admin INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sync_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at TEXT,
    context TEXT NOT NULL,
    posted_from TEXT,
    posted_to TEXT,
    api_calls INTEGER NOT NULL DEFAULT 0,
    records_fetched INTEGER NOT NULL DEFAULT 0,
    rate_limited INTEGER NOT NULL DEFAULT 0,
    error_message TEXT
);

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS saved_searches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    search_query TEXT,
    naics_code TEXT,
    opp_type TEXT,
    set_aside TEXT,
    state TEXT,
    department TEXT,
    active_only INTEGER NOT NULL DEFAULT 1,
    include_keywords TEXT,
    exclude_keywords TEXT,
    match_all INTEGER NOT NULL DEFAULT 0,
    webhook_url TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_saved_searches_user ON saved_searches(user_id);

CREATE TABLE IF NOT EXISTS alerts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    saved_search_id INTEGER NOT NULL REFERENCES saved_searches(id) ON DELETE CASCADE,
    opportunity_id TEXT NOT NULL REFERENCES opportunities(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(saved_search_id, opportunity_id)
);

CREATE INDEX IF NOT EXISTS idx_alerts_search ON alerts(saved_search_id);
CREATE INDEX IF NOT EXISTS idx_alerts_opp ON alerts(opportunity_id);

CREATE TABLE IF NOT EXISTS deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alert_id INTEGER NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    status_code INTEGER,
    error_message TEXT,
    delivered_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_deliveries_alert ON deliveries(alert_id);

CREATE TABLE IF NOT EXISTS saved_filters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    search_query TEXT,
    naics_code TEXT,
    opp_type TEXT,
    set_aside TEXT,
    state TEXT,
    department TEXT,
    active_only INTEGER NOT NULL DEFAULT 0,
    response_deadline TEXT,
    is_default INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_saved_filters_user ON saved_filters(user_id);
