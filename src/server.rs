use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::Arc;

use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::Json,
    routing::get,
    Router,
};
use rusqlite::Connection;
use serde::{Deserialize, Serialize};
use tower_http::cors::CorsLayer;

use govscout_lib::db;

struct AppState {
    db_path: PathBuf,
}

fn open_db(state: &AppState) -> Result<Connection, StatusCode> {
    let conn = Connection::open(&state.db_path).map_err(|e| {
        eprintln!("Failed to open database: {e}");
        StatusCode::INTERNAL_SERVER_ERROR
    })?;
    db::configure_pragmas(&conn).map_err(|e| {
        eprintln!("Failed to set pragmas: {e}");
        StatusCode::INTERNAL_SERVER_ERROR
    })?;
    conn.execute_batch(
        "CREATE TABLE IF NOT EXISTS api_call_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL DEFAULT (datetime('now')),
            context TEXT NOT NULL,
            posted_from TEXT,
            posted_to TEXT,
            api_calls INTEGER NOT NULL,
            records_fetched INTEGER NOT NULL,
            rate_limited INTEGER NOT NULL DEFAULT 0,
            error_message TEXT
        );",
    )
    .map_err(|e| {
        eprintln!("Failed to ensure api_call_log table: {e}");
        StatusCode::INTERNAL_SERVER_ERROR
    })?;
    Ok(conn)
}

// --- Request / Response types ---

#[derive(Deserialize)]
struct ListParams {
    search: Option<String>,
    naics_code: Option<String>,
    opp_type: Option<String>,
    set_aside: Option<String>,
    state: Option<String>,
    department: Option<String>,
    date_from: Option<String>,
    date_to: Option<String>,
    active_only: Option<bool>,
    limit: Option<u32>,
    offset: Option<u32>,
}

#[derive(Serialize)]
struct ListResponse {
    total: u64,
    limit: u32,
    offset: u32,
    opportunities: Vec<OpportunityRow>,
}

#[derive(Serialize)]
struct OpportunityRow {
    notice_id: Option<String>,
    title: Option<String>,
    solicitation_number: Option<String>,
    department: Option<String>,
    sub_tier: Option<String>,
    office: Option<String>,
    opp_type: Option<String>,
    base_type: Option<String>,
    posted_date: Option<String>,
    response_deadline: Option<String>,
    naics_code: Option<String>,
    set_aside: Option<String>,
    set_aside_description: Option<String>,
    description: Option<String>,
    active: Option<String>,
    ui_link: Option<String>,
    pop_state_code: Option<String>,
    pop_state_name: Option<String>,
}

#[derive(Serialize)]
struct DetailResponse {
    opportunity: OpportunityDetail,
    contacts: Vec<ContactRow>,
}

#[derive(Serialize)]
struct OpportunityDetail {
    notice_id: Option<String>,
    title: Option<String>,
    solicitation_number: Option<String>,
    department: Option<String>,
    sub_tier: Option<String>,
    office: Option<String>,
    full_parent_path_name: Option<String>,
    organization_type: Option<String>,
    opp_type: Option<String>,
    base_type: Option<String>,
    posted_date: Option<String>,
    response_deadline: Option<String>,
    archive_date: Option<String>,
    naics_code: Option<String>,
    classification_code: Option<String>,
    set_aside: Option<String>,
    set_aside_description: Option<String>,
    description: Option<String>,
    ui_link: Option<String>,
    active: Option<String>,
    resource_links: Option<Vec<String>>,
    award_amount: Option<String>,
    award_date: Option<String>,
    award_number: Option<String>,
    awardee_name: Option<String>,
    awardee_uei_sam: Option<String>,
    pop_state_code: Option<String>,
    pop_state_name: Option<String>,
    pop_city_name: Option<String>,
    pop_country_name: Option<String>,
    pop_zip: Option<String>,
}

#[derive(Serialize)]
struct ContactRow {
    contact_type: Option<String>,
    full_name: Option<String>,
    email: Option<String>,
    phone: Option<String>,
    title: Option<String>,
}

#[derive(Serialize)]
struct StatsResponse {
    total_opportunities: u64,
    naics_codes: Vec<FilterOption>,
    opp_types: Vec<FilterOption>,
    set_asides: Vec<FilterOption>,
    states: Vec<FilterOption>,
    departments: Vec<FilterOption>,
}

#[derive(Serialize)]
struct FilterOption {
    value: String,
    count: u64,
}

// --- Query builder helper ---

struct QueryBuilder {
    clauses: Vec<String>,
    params: Vec<String>,
}

impl QueryBuilder {
    fn new() -> Self {
        Self {
            clauses: Vec::new(),
            params: Vec::new(),
        }
    }

    fn add_like_search(&mut self, search: &str) {
        let escaped = search
            .replace('\\', "\\\\")
            .replace('%', "\\%")
            .replace('_', "\\_");
        let pattern = format!("%{escaped}%");
        let i1 = self.params.len() + 1;
        let i2 = i1 + 1;
        let i3 = i1 + 2;
        self.clauses.push(format!(
            "(title LIKE ?{i1} ESCAPE '\\' OR solicitation_number LIKE ?{i2} ESCAPE '\\' OR department LIKE ?{i3} ESCAPE '\\')"
        ));
        self.params.push(pattern.clone());
        self.params.push(pattern.clone());
        self.params.push(pattern);
    }

    fn add_eq(&mut self, column: &str, value: &str) {
        let idx = self.params.len() + 1;
        self.clauses.push(format!("{column} = ?{idx}"));
        self.params.push(value.to_string());
    }

    fn add_gte(&mut self, column: &str, value: &str) {
        let idx = self.params.len() + 1;
        self.clauses.push(format!("{column} >= ?{idx}"));
        self.params.push(value.to_string());
    }

    fn add_lte(&mut self, column: &str, value: &str) {
        let idx = self.params.len() + 1;
        self.clauses.push(format!("{column} <= ?{idx}"));
        self.params.push(value.to_string());
    }

    fn add_in(&mut self, column: &str, values: &[&str]) {
        if values.is_empty() {
            return;
        }
        let placeholders: Vec<String> = values
            .iter()
            .map(|v| {
                let idx = self.params.len() + 1;
                self.params.push(v.to_string());
                format!("?{idx}")
            })
            .collect();
        self.clauses
            .push(format!("{column} IN ({})", placeholders.join(", ")));
    }

    fn add_literal(&mut self, clause: &str) {
        self.clauses.push(clause.to_string());
    }

    fn where_sql(&self) -> String {
        if self.clauses.is_empty() {
            String::new()
        } else {
            format!("WHERE {}", self.clauses.join(" AND "))
        }
    }

    fn params_as_tosql(&self) -> Vec<&dyn rusqlite::types::ToSql> {
        self.params
            .iter()
            .map(|s| s as &dyn rusqlite::types::ToSql)
            .collect()
    }

    fn next_param_idx(&self) -> usize {
        self.params.len() + 1
    }
}

// --- Handlers ---

async fn health() -> &'static str {
    "ok"
}

async fn list_opportunities(
    State(state): State<Arc<AppState>>,
    Query(params): Query<ListParams>,
) -> Result<Json<ListResponse>, StatusCode> {
    let conn = open_db(&state)?;

    let limit = params.limit.unwrap_or(25).min(100);
    let offset = params.offset.unwrap_or(0);

    let mut qb = QueryBuilder::new();

    if let Some(ref search) = params.search {
        if !search.is_empty() {
            qb.add_like_search(search);
        }
    }
    if let Some(ref v) = params.naics_code {
        if v.contains(',') {
            let codes: Vec<&str> = v.split(',').map(|s| s.trim()).collect();
            qb.add_in("naics_code", &codes);
        } else {
            qb.add_eq("naics_code", v);
        }
    }
    if let Some(ref v) = params.opp_type {
        if v.contains(',') {
            let vals: Vec<&str> = v.split(',').map(|s| s.trim()).collect();
            qb.add_in("opp_type", &vals);
        } else {
            qb.add_eq("opp_type", v);
        }
    }
    if let Some(ref v) = params.set_aside {
        if v.contains(',') {
            let vals: Vec<&str> = v.split(',').map(|s| s.trim()).collect();
            qb.add_in("set_aside", &vals);
        } else {
            qb.add_eq("set_aside", v);
        }
    }
    if let Some(ref v) = params.state {
        if v.contains(',') {
            let vals: Vec<&str> = v.split(',').map(|s| s.trim()).collect();
            qb.add_in("pop_state_code", &vals);
        } else {
            qb.add_eq("pop_state_code", v);
        }
    }
    if let Some(ref v) = params.department {
        if v.contains(',') {
            let vals: Vec<&str> = v.split(',').map(|s| s.trim()).collect();
            qb.add_in("department", &vals);
        } else {
            qb.add_eq("department", v);
        }
    }
    if let Some(ref v) = params.date_from {
        qb.add_gte("posted_date", v);
    }
    if let Some(ref v) = params.date_to {
        qb.add_lte("posted_date", v);
    }
    if params.active_only.unwrap_or(false) {
        qb.add_literal("active = 'Yes'");
    }

    let where_sql = qb.where_sql();

    // Count
    let count_sql = format!("SELECT COUNT(*) FROM opportunities {where_sql}");
    let count_params = qb.params_as_tosql();
    let total: u64 = conn
        .prepare(&count_sql)
        .map_err(|e| {
            eprintln!("Failed to prepare count query: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?
        .query_row(count_params.as_slice(), |row| row.get(0))
        .map_err(|e| {
            eprintln!("Failed to execute count query: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?;

    // Data
    let li = qb.next_param_idx();
    let oi = li + 1;
    let data_sql = format!(
        "SELECT notice_id, title, solicitation_number, department, sub_tier, office,
                opp_type, base_type, posted_date, response_deadline, naics_code,
                set_aside, set_aside_description, active, ui_link, pop_state_code, pop_state_name,
                description
         FROM opportunities {where_sql}
         ORDER BY posted_date DESC
         LIMIT ?{li} OFFSET ?{oi}"
    );

    let limit_s = limit.to_string();
    let offset_s = offset.to_string();
    let mut data_params = qb.params_as_tosql();
    data_params.push(&limit_s);
    data_params.push(&offset_s);

    let mut stmt = conn.prepare(&data_sql).map_err(|e| {
        eprintln!("Failed to prepare data query: {e}");
        StatusCode::INTERNAL_SERVER_ERROR
    })?;

    let opportunities: Vec<OpportunityRow> = stmt
        .query_map(data_params.as_slice(), |row| {
            Ok(OpportunityRow {
                notice_id: row.get(0)?,
                title: row.get(1)?,
                solicitation_number: row.get(2)?,
                department: row.get(3)?,
                sub_tier: row.get(4)?,
                office: row.get(5)?,
                opp_type: row.get(6)?,
                base_type: row.get(7)?,
                posted_date: row.get(8)?,
                response_deadline: row.get(9)?,
                naics_code: row.get(10)?,
                set_aside: row.get(11)?,
                set_aside_description: row.get(12)?,
                description: row.get(17)?,
                active: row.get(13)?,
                ui_link: row.get(14)?,
                pop_state_code: row.get(15)?,
                pop_state_name: row.get(16)?,
            })
        })
        .map_err(|e| {
            eprintln!("Failed to query opportunities: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?
        .filter_map(|r| {
            r.map_err(|e| eprintln!("Failed to read opportunity row: {e}"))
                .ok()
        })
        .collect();

    Ok(Json(ListResponse {
        total,
        limit,
        offset,
        opportunities,
    }))
}

async fn get_opportunity(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
) -> Result<Json<DetailResponse>, StatusCode> {
    let conn = open_db(&state)?;

    let opp = conn
        .prepare(
            "SELECT notice_id, title, solicitation_number, department, sub_tier, office,
                    full_parent_path_name, organization_type, opp_type, base_type,
                    posted_date, response_deadline, archive_date, naics_code,
                    classification_code, set_aside, set_aside_description, description,
                    ui_link, active, resource_links,
                    award_amount, award_date, award_number, awardee_name, awardee_uei_sam,
                    pop_state_code, pop_state_name, pop_city_name, pop_country_name, pop_zip
             FROM opportunities WHERE notice_id = ?1",
        )
        .map_err(|e| {
            eprintln!("Failed to prepare opportunity query: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?
        .query_row(rusqlite::params![id], |row| {
            let resource_links_str: Option<String> = row.get(20)?;
            let resource_links: Option<Vec<String>> =
                resource_links_str.and_then(|s| serde_json::from_str(&s).ok());

            Ok(OpportunityDetail {
                notice_id: row.get(0)?,
                title: row.get(1)?,
                solicitation_number: row.get(2)?,
                department: row.get(3)?,
                sub_tier: row.get(4)?,
                office: row.get(5)?,
                full_parent_path_name: row.get(6)?,
                organization_type: row.get(7)?,
                opp_type: row.get(8)?,
                base_type: row.get(9)?,
                posted_date: row.get(10)?,
                response_deadline: row.get(11)?,
                archive_date: row.get(12)?,
                naics_code: row.get(13)?,
                classification_code: row.get(14)?,
                set_aside: row.get(15)?,
                set_aside_description: row.get(16)?,
                description: row.get(17)?,
                ui_link: row.get(18)?,
                active: row.get(19)?,
                resource_links,
                award_amount: row.get(21)?,
                award_date: row.get(22)?,
                award_number: row.get(23)?,
                awardee_name: row.get(24)?,
                awardee_uei_sam: row.get(25)?,
                pop_state_code: row.get(26)?,
                pop_state_name: row.get(27)?,
                pop_city_name: row.get(28)?,
                pop_country_name: row.get(29)?,
                pop_zip: row.get(30)?,
            })
        })
        .map_err(|e| {
            eprintln!("Opportunity not found (id={id}): {e}");
            StatusCode::NOT_FOUND
        })?;

    let mut stmt = conn
        .prepare(
            "SELECT contact_type, full_name, email, phone, title
             FROM contacts WHERE notice_id = ?1",
        )
        .map_err(|e| {
            eprintln!("Failed to prepare contacts query: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?;

    let contacts: Vec<ContactRow> = stmt
        .query_map(rusqlite::params![id], |row| {
            Ok(ContactRow {
                contact_type: row.get(0)?,
                full_name: row.get(1)?,
                email: row.get(2)?,
                phone: row.get(3)?,
                title: row.get(4)?,
            })
        })
        .map_err(|e| {
            eprintln!("Failed to query contacts: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?
        .filter_map(|r| {
            r.map_err(|e| eprintln!("Failed to read contact row: {e}"))
                .ok()
        })
        .collect();

    Ok(Json(DetailResponse {
        opportunity: opp,
        contacts,
    }))
}

async fn get_stats(State(state): State<Arc<AppState>>) -> Result<Json<StatsResponse>, StatusCode> {
    let conn = open_db(&state)?;

    let total_opportunities: u64 = conn
        .query_row("SELECT COUNT(*) FROM opportunities", [], |row| row.get(0))
        .map_err(|e| {
            eprintln!("Failed to count opportunities: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?;

    let query_distinct = |col: &str| -> Result<Vec<FilterOption>, StatusCode> {
        let sql = format!(
            "SELECT {col}, COUNT(*) as cnt FROM opportunities \
             WHERE {col} IS NOT NULL AND {col} != '' \
             GROUP BY {col} ORDER BY cnt DESC"
        );
        let mut stmt = conn.prepare(&sql).map_err(|e| {
            eprintln!("Failed to prepare stats query for {col}: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?;
        let rows = stmt
            .query_map([], |row| {
                Ok(FilterOption {
                    value: row.get(0)?,
                    count: row.get(1)?,
                })
            })
            .map_err(|e| {
                eprintln!("Failed to query stats for {col}: {e}");
                StatusCode::INTERNAL_SERVER_ERROR
            })?;
        Ok(rows
            .filter_map(|r| {
                r.map_err(|e| eprintln!("Failed to read stats row: {e}"))
                    .ok()
            })
            .collect())
    };

    Ok(Json(StatsResponse {
        total_opportunities,
        naics_codes: query_distinct("naics_code")?,
        opp_types: query_distinct("opp_type")?,
        set_asides: query_distinct("set_aside")?,
        states: query_distinct("pop_state_code")?,
        departments: query_distinct("department")?,
    }))
}

#[derive(Deserialize)]
struct ApiCallsParams {
    limit: Option<u32>,
}

#[derive(Serialize)]
struct ApiCallLogEntry {
    id: i64,
    timestamp: String,
    context: String,
    posted_from: Option<String>,
    posted_to: Option<String>,
    api_calls: i64,
    records_fetched: i64,
    rate_limited: bool,
    error_message: Option<String>,
}

async fn list_api_calls(
    State(state): State<Arc<AppState>>,
    Query(params): Query<ApiCallsParams>,
) -> Result<Json<Vec<ApiCallLogEntry>>, StatusCode> {
    let conn = open_db(&state)?;
    let limit = params.limit.unwrap_or(50).min(200);

    let mut stmt = conn
        .prepare(
            "SELECT id, timestamp, context, posted_from, posted_to, api_calls, records_fetched, rate_limited, error_message
             FROM api_call_log ORDER BY id DESC LIMIT ?1",
        )
        .map_err(|e| {
            eprintln!("Failed to prepare api_call_log query: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?;

    let entries: Vec<ApiCallLogEntry> = stmt
        .query_map(rusqlite::params![limit], |row| {
            Ok(ApiCallLogEntry {
                id: row.get(0)?,
                timestamp: row.get(1)?,
                context: row.get(2)?,
                posted_from: row.get(3)?,
                posted_to: row.get(4)?,
                api_calls: row.get(5)?,
                records_fetched: row.get(6)?,
                rate_limited: row.get::<_, i32>(7)? != 0,
                error_message: row.get(8)?,
            })
        })
        .map_err(|e| {
            eprintln!("Failed to query api_call_log: {e}");
            StatusCode::INTERNAL_SERVER_ERROR
        })?
        .filter_map(|r| {
            r.map_err(|e| eprintln!("Failed to read api_call_log row: {e}"))
                .ok()
        })
        .collect();

    Ok(Json(entries))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_query_builder_empty() {
        let qb = QueryBuilder::new();
        assert_eq!(qb.where_sql(), "");
        assert!(qb.params_as_tosql().is_empty());
        assert_eq!(qb.next_param_idx(), 1);
    }

    #[test]
    fn test_query_builder_add_eq() {
        let mut qb = QueryBuilder::new();
        qb.add_eq("opp_type", "Solicitation");
        assert_eq!(qb.where_sql(), "WHERE opp_type = ?1");
        assert_eq!(qb.params.len(), 1);
        assert_eq!(qb.params[0], "Solicitation");
    }

    #[test]
    fn test_query_builder_multiple_clauses() {
        let mut qb = QueryBuilder::new();
        qb.add_eq("opp_type", "Solicitation");
        qb.add_eq("naics_code", "541512");
        assert_eq!(qb.where_sql(), "WHERE opp_type = ?1 AND naics_code = ?2");
        assert_eq!(qb.params.len(), 2);
    }

    #[test]
    fn test_query_builder_like_search() {
        let mut qb = QueryBuilder::new();
        qb.add_like_search("cloud");
        let sql = qb.where_sql();
        assert!(sql.contains("title LIKE ?1"));
        assert!(sql.contains("solicitation_number LIKE ?2"));
        assert!(sql.contains("department LIKE ?3"));
        assert_eq!(qb.params.len(), 3);
        assert_eq!(qb.params[0], "%cloud%");
    }

    #[test]
    fn test_query_builder_add_in() {
        let mut qb = QueryBuilder::new();
        qb.add_in("naics_code", &["541512", "541511"]);
        assert_eq!(qb.where_sql(), "WHERE naics_code IN (?1, ?2)");
        assert_eq!(qb.params.len(), 2);
    }

    #[test]
    fn test_query_builder_add_gte_lte() {
        let mut qb = QueryBuilder::new();
        qb.add_gte("posted_date", "2025-01-01");
        qb.add_lte("posted_date", "2025-12-31");
        assert_eq!(
            qb.where_sql(),
            "WHERE posted_date >= ?1 AND posted_date <= ?2"
        );
    }

    #[test]
    fn test_query_builder_add_literal() {
        let mut qb = QueryBuilder::new();
        qb.add_literal("active = 'Yes'");
        assert_eq!(qb.where_sql(), "WHERE active = 'Yes'");
        assert!(qb.params.is_empty());
    }

    #[test]
    fn test_query_builder_next_param_idx() {
        let mut qb = QueryBuilder::new();
        assert_eq!(qb.next_param_idx(), 1);
        qb.add_eq("a", "1");
        assert_eq!(qb.next_param_idx(), 2);
        qb.add_eq("b", "2");
        assert_eq!(qb.next_param_idx(), 3);
    }

    #[test]
    fn test_query_builder_combined() {
        let mut qb = QueryBuilder::new();
        qb.add_like_search("test");
        qb.add_eq("opp_type", "Solicitation");
        qb.add_literal("active = 'Yes'");
        let sql = qb.where_sql();
        assert!(sql.starts_with("WHERE "));
        assert!(sql.contains("title LIKE"));
        assert!(sql.contains("opp_type = ?4"));
        assert!(sql.contains("active = 'Yes'"));
        assert_eq!(qb.params.len(), 4);
    }
}

#[tokio::main]
async fn main() {
    dotenvy::dotenv().ok();

    let db_path = db::resolve_db_path().expect("Failed to resolve database path");
    if !db_path.exists() {
        eprintln!(
            "Warning: Database not found at {}. Run `govscout search` first to populate data.",
            db_path.display()
        );
    }

    let state = Arc::new(AppState { db_path });

    let port: u16 = std::env::var("PORT")
        .ok()
        .and_then(|p| p.parse().ok())
        .unwrap_or(3001);

    let app = Router::new()
        .route("/health", get(health))
        .route("/api/opportunities", get(list_opportunities))
        .route("/api/opportunities/{id}", get(get_opportunity))
        .route("/api/stats", get(get_stats))
        .route("/api/api-calls", get(list_api_calls))
        .layer(CorsLayer::permissive())
        .with_state(state);

    let addr = SocketAddr::from(([0, 0, 0, 0], port));
    eprintln!("GovScout API server listening on http://localhost:{port}");

    let listener = tokio::net::TcpListener::bind(addr)
        .await
        .unwrap_or_else(|e| {
            eprintln!("Failed to bind to {addr}: {e}");
            std::process::exit(1);
        });
    if let Err(e) = axum::serve(listener, app).await {
        eprintln!("Server error: {e}");
        std::process::exit(1);
    }
}
