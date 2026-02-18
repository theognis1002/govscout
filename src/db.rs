use anyhow::{Context, Result};
use rusqlite::Connection;
use rusqlite::OptionalExtension;
use std::path::PathBuf;

use crate::api::{ApiResponse, Opportunity};

pub struct Database {
    conn: Connection,
}

pub struct ApiCallLogRow {
    pub id: i64,
    pub timestamp: String,
    pub context: String,
    pub posted_from: Option<String>,
    pub posted_to: Option<String>,
    pub api_calls: i64,
    pub records_fetched: i64,
    pub rate_limited: bool,
    pub error_message: Option<String>,
}

impl Database {
    pub fn open() -> Result<Self> {
        let path = resolve_db_path()?;
        if let Some(parent) = path.parent() {
            std::fs::create_dir_all(parent).with_context(|| {
                format!("Failed to create database directory: {}", parent.display())
            })?;
        }

        let conn = Connection::open(&path)
            .with_context(|| format!("Failed to open database at {}", path.display()))?;

        configure_pragmas(&conn)?;

        let db = Self { conn };
        db.init_schema()?;
        Ok(db)
    }

    #[cfg(test)]
    pub fn open_in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory().context("Failed to open in-memory database")?;

        conn.execute_batch("PRAGMA foreign_keys=ON;")
            .context("Failed to set foreign_keys pragma")?;

        let db = Self { conn };
        db.init_schema()?;
        Ok(db)
    }

    fn init_schema(&self) -> Result<()> {
        self.conn
            .execute_batch(
                "CREATE TABLE IF NOT EXISTS opportunities (
                    notice_id TEXT NOT NULL PRIMARY KEY,
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
                    active TEXT,
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
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
                );

                CREATE TABLE IF NOT EXISTS contacts (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    notice_id TEXT NOT NULL REFERENCES opportunities(notice_id) ON DELETE CASCADE,
                    contact_type TEXT,
                    full_name TEXT,
                    email TEXT,
                    phone TEXT,
                    title TEXT,
                    created_at TEXT NOT NULL DEFAULT (datetime('now')),
                    modified_at TEXT NOT NULL DEFAULT (datetime('now'))
                );

                CREATE INDEX IF NOT EXISTS idx_opp_posted_date ON opportunities(posted_date);
                CREATE INDEX IF NOT EXISTS idx_opp_naics_code ON opportunities(naics_code);
                CREATE INDEX IF NOT EXISTS idx_opp_opp_type ON opportunities(opp_type);
                CREATE INDEX IF NOT EXISTS idx_opp_base_type ON opportunities(base_type);
                CREATE INDEX IF NOT EXISTS idx_opp_set_aside ON opportunities(set_aside);
                CREATE INDEX IF NOT EXISTS idx_opp_active ON opportunities(active);
                CREATE INDEX IF NOT EXISTS idx_opp_pop_state ON opportunities(pop_state_code);
                CREATE INDEX IF NOT EXISTS idx_opp_naics_type ON opportunities(naics_code, opp_type);
                CREATE INDEX IF NOT EXISTS idx_contacts_notice ON contacts(notice_id);

                CREATE TABLE IF NOT EXISTS sync_state (
                    key TEXT PRIMARY KEY,
                    value TEXT NOT NULL
                );

                CREATE TABLE IF NOT EXISTS api_call_log (
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
            .context("Failed to initialize database schema")?;

        Ok(())
    }

    pub fn get_sync_state(&self, key: &str) -> Result<Option<String>> {
        let result = self
            .conn
            .query_row(
                "SELECT value FROM sync_state WHERE key = ?1",
                rusqlite::params![key],
                |row| row.get(0),
            )
            .optional()
            .context("Failed to query sync_state")?;
        Ok(result)
    }

    pub fn set_sync_state(&self, key: &str, value: &str) -> Result<()> {
        self.conn
            .execute(
                "INSERT INTO sync_state (key, value) VALUES (?1, ?2)
                 ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                rusqlite::params![key, value],
            )
            .context("Failed to set sync_state")?;
        Ok(())
    }

    pub fn get_earliest_posted_date(&self) -> Result<Option<String>> {
        let result = self
            .conn
            .query_row(
                "SELECT MIN(posted_date) FROM opportunities WHERE posted_date IS NOT NULL",
                [],
                |row| row.get(0),
            )
            .optional()
            .context("Failed to query earliest posted_date")?;
        // optional() wraps the row, but the value itself can be NULL
        Ok(result.flatten())
    }

    pub fn upsert_opportunity(&mut self, opp: &Opportunity) -> Result<()> {
        let tx = self.conn.transaction()?;
        Self::upsert_opportunity_inner(&tx, opp)?;
        tx.commit().context("Failed to commit transaction")?;
        Ok(())
    }

    pub fn upsert_opportunities(&mut self, response: &ApiResponse) -> Result<()> {
        let opps = match &response.opportunities_data {
            Some(opps) => opps,
            None => return Ok(()),
        };

        let tx = self.conn.transaction()?;
        for opp in opps {
            Self::upsert_opportunity_inner(&tx, opp)?;
        }
        tx.commit().context("Failed to commit transaction")?;

        Ok(())
    }

    #[allow(clippy::too_many_arguments)]
    pub fn log_api_call(
        &self,
        context: &str,
        posted_from: Option<&str>,
        posted_to: Option<&str>,
        api_calls: u32,
        records_fetched: usize,
        rate_limited: bool,
        error_message: Option<&str>,
    ) -> Result<()> {
        self.conn
            .execute(
                "INSERT INTO api_call_log (context, posted_from, posted_to, api_calls, records_fetched, rate_limited, error_message)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)",
                rusqlite::params![
                    context,
                    posted_from,
                    posted_to,
                    api_calls,
                    records_fetched as i64,
                    rate_limited as i32,
                    error_message,
                ],
            )
            .context("Failed to insert api_call_log")?;

        // Prune to keep latest 200 rows
        self.conn
            .execute(
                "DELETE FROM api_call_log WHERE id NOT IN (SELECT id FROM api_call_log ORDER BY id DESC LIMIT 200)",
                [],
            )
            .context("Failed to prune api_call_log")?;

        Ok(())
    }

    pub fn get_api_call_logs(&self, limit: u32) -> Result<Vec<ApiCallLogRow>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, timestamp, context, posted_from, posted_to, api_calls, records_fetched, rate_limited, error_message
             FROM api_call_log ORDER BY id DESC LIMIT ?1",
        )?;

        let rows = stmt
            .query_map(rusqlite::params![limit], |row| {
                Ok(ApiCallLogRow {
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
            })?
            .filter_map(|r| r.ok())
            .collect();

        Ok(rows)
    }

    fn upsert_opportunity_inner(conn: &Connection, opp: &Opportunity) -> Result<()> {
        let notice_id = match opp.notice_id.as_deref() {
            Some(id) => id,
            None => return Ok(()),
        };

        let resource_links_json = opp
            .resource_links
            .as_ref()
            .map(|links| serde_json::to_string(links).unwrap_or_default());

        let (award_amount, award_date, award_number, awardee_name, awardee_duns, awardee_uei_sam) =
            match &opp.award {
                Some(award) => {
                    let (name, duns, uei) = match &award.awardee {
                        Some(a) => (a.name.as_deref(), a.duns.as_deref(), a.uei_sam.as_deref()),
                        None => (None, None, None),
                    };
                    (
                        award.amount.as_deref(),
                        award.date.as_deref(),
                        award.number.as_deref(),
                        name,
                        duns,
                        uei,
                    )
                }
                None => (None, None, None, None, None, None),
            };

        let (
            pop_state_code,
            pop_state_name,
            pop_city_code,
            pop_city_name,
            pop_country_code,
            pop_country_name,
            pop_zip,
        ) = match &opp.place_of_performance {
            Some(pop) => (
                pop.state.as_ref().and_then(|v| v.code.as_deref()),
                pop.state.as_ref().and_then(|v| v.name.as_deref()),
                pop.city.as_ref().and_then(|v| v.code.as_deref()),
                pop.city.as_ref().and_then(|v| v.name.as_deref()),
                pop.country.as_ref().and_then(|v| v.code.as_deref()),
                pop.country.as_ref().and_then(|v| v.name.as_deref()),
                pop.zip.as_deref(),
            ),
            None => (None, None, None, None, None, None, None),
        };

        conn.execute(
            "INSERT INTO opportunities (
                notice_id, title, solicitation_number, department, sub_tier, office,
                full_parent_path_name, organization_type, opp_type, base_type,
                posted_date, response_deadline, archive_date, naics_code,
                classification_code, set_aside, set_aside_description, description,
                ui_link, active, resource_links,
                award_amount, award_date, award_number,
                awardee_name, awardee_duns, awardee_uei_sam,
                pop_state_code, pop_state_name, pop_city_code, pop_city_name,
                pop_country_code, pop_country_name, pop_zip
            ) VALUES (
                ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10,
                ?11, ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19, ?20, ?21,
                ?22, ?23, ?24, ?25, ?26, ?27,
                ?28, ?29, ?30, ?31, ?32, ?33, ?34
            )
            ON CONFLICT(notice_id) DO UPDATE SET
                title = excluded.title,
                solicitation_number = excluded.solicitation_number,
                department = excluded.department,
                sub_tier = excluded.sub_tier,
                office = excluded.office,
                full_parent_path_name = excluded.full_parent_path_name,
                organization_type = excluded.organization_type,
                opp_type = excluded.opp_type,
                base_type = excluded.base_type,
                posted_date = excluded.posted_date,
                response_deadline = excluded.response_deadline,
                archive_date = excluded.archive_date,
                naics_code = excluded.naics_code,
                classification_code = excluded.classification_code,
                set_aside = excluded.set_aside,
                set_aside_description = excluded.set_aside_description,
                description = excluded.description,
                ui_link = excluded.ui_link,
                active = excluded.active,
                resource_links = excluded.resource_links,
                award_amount = excluded.award_amount,
                award_date = excluded.award_date,
                award_number = excluded.award_number,
                awardee_name = excluded.awardee_name,
                awardee_duns = excluded.awardee_duns,
                awardee_uei_sam = excluded.awardee_uei_sam,
                pop_state_code = excluded.pop_state_code,
                pop_state_name = excluded.pop_state_name,
                pop_city_code = excluded.pop_city_code,
                pop_city_name = excluded.pop_city_name,
                pop_country_code = excluded.pop_country_code,
                pop_country_name = excluded.pop_country_name,
                pop_zip = excluded.pop_zip,
                modified_at = datetime('now')",
            rusqlite::params![
                notice_id,
                opp.title,
                opp.solicitation_number,
                opp.department,
                opp.sub_tier,
                opp.office,
                opp.full_parent_path_name,
                opp.organization_type,
                opp.opp_type,
                opp.base_type,
                opp.posted_date,
                opp.response_deadline,
                opp.archive_date,
                opp.naics_code,
                opp.classification_code,
                opp.set_aside,
                opp.set_aside_description,
                opp.description,
                opp.ui_link,
                opp.active,
                resource_links_json,
                award_amount,
                award_date,
                award_number,
                awardee_name,
                awardee_duns,
                awardee_uei_sam,
                pop_state_code,
                pop_state_name,
                pop_city_code,
                pop_city_name,
                pop_country_code,
                pop_country_name,
                pop_zip,
            ],
        )
        .context("Failed to upsert opportunity")?;

        // Replace contacts: delete existing, then insert new
        conn.execute(
            "DELETE FROM contacts WHERE notice_id = ?1",
            rusqlite::params![notice_id],
        )
        .context("Failed to delete existing contacts")?;

        if let Some(contacts) = &opp.point_of_contact {
            let mut stmt = conn.prepare(
                "INSERT INTO contacts (notice_id, contact_type, full_name, email, phone, title)
                 VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
            )?;
            for contact in contacts {
                stmt.execute(rusqlite::params![
                    notice_id,
                    contact.contact_type,
                    contact.full_name,
                    contact.email,
                    contact.phone,
                    contact.title,
                ])?;
            }
        }

        Ok(())
    }
}

pub fn configure_pragmas(conn: &Connection) -> Result<()> {
    conn.execute_batch(
        "PRAGMA journal_mode=WAL;
         PRAGMA synchronous=NORMAL;
         PRAGMA foreign_keys=ON;
         PRAGMA busy_timeout=5000;",
    )
    .context("Failed to set database pragmas")?;
    Ok(())
}

pub fn resolve_db_path() -> Result<PathBuf> {
    if let Ok(path) = std::env::var("GOVSCOUT_DB") {
        return Ok(PathBuf::from(path));
    }

    Ok(PathBuf::from("govscout.db"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::*;

    fn make_opportunity(notice_id: &str, title: &str) -> Opportunity {
        Opportunity {
            notice_id: Some(notice_id.into()),
            title: Some(title.into()),
            solicitation_number: None,
            department: None,
            sub_tier: None,
            office: None,
            full_parent_path_name: None,
            organization_type: None,
            opp_type: None,
            base_type: None,
            posted_date: None,
            response_deadline: None,
            archive_date: None,
            naics_code: None,
            classification_code: None,
            set_aside: None,
            set_aside_description: None,
            description: None,
            ui_link: None,
            resource_links: None,
            award: None,
            point_of_contact: None,
            place_of_performance: None,
            active: None,
        }
    }

    #[test]
    fn test_schema_creation() {
        let db = Database::open_in_memory().unwrap();
        let tables: Vec<String> = db
            .conn
            .prepare("SELECT name FROM sqlite_master WHERE type='table' AND name IN ('opportunities', 'contacts') ORDER BY name")
            .unwrap()
            .query_map([], |row| row.get(0))
            .unwrap()
            .map(|r| r.unwrap())
            .collect();
        assert_eq!(tables, vec!["contacts", "opportunities"]);

        let idx_count: i64 = db
            .conn
            .query_row(
                "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name LIKE 'idx_opp_%'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert!(idx_count >= 7);
    }

    #[test]
    fn test_upsert_full_opportunity() {
        let mut db = Database::open_in_memory().unwrap();
        let opp = Opportunity {
            notice_id: Some("FULL-001".into()),
            title: Some("Full Test".into()),
            solicitation_number: Some("SOL-001".into()),
            department: Some("DOD".into()),
            sub_tier: Some("Army".into()),
            office: Some("ACC".into()),
            full_parent_path_name: Some("DOD.Army.ACC".into()),
            organization_type: Some("DEPT".into()),
            opp_type: Some("Solicitation".into()),
            base_type: Some("Presolicitation".into()),
            posted_date: Some("01/15/2026".into()),
            response_deadline: Some("02/15/2026".into()),
            archive_date: Some("03/15/2026".into()),
            naics_code: Some("541512".into()),
            classification_code: Some("D302".into()),
            set_aside: Some("SBA".into()),
            set_aside_description: Some("Total Small Business".into()),
            description: Some("A description".into()),
            ui_link: Some("https://sam.gov/opp/full".into()),
            active: Some("Yes".into()),
            resource_links: Some(vec!["https://example.com/doc.pdf".into()]),
            award: Some(Award {
                amount: Some("500000".into()),
                date: Some("2026-01-01".into()),
                number: Some("AWD-001".into()),
                awardee: Some(Awardee {
                    name: Some("Acme".into()),
                    duns: Some("111".into()),
                    uei_sam: Some("UEI111".into()),
                }),
            }),
            point_of_contact: Some(vec![PointOfContact {
                contact_type: Some("Primary".into()),
                full_name: Some("Jane".into()),
                email: Some("jane@gov.gov".into()),
                phone: Some("555-0000".into()),
                title: Some("CO".into()),
            }]),
            place_of_performance: Some(PlaceOfPerformance {
                state: Some(PlaceValue {
                    code: Some("VA".into()),
                    name: Some("Virginia".into()),
                }),
                city: Some(PlaceValue {
                    code: Some("001".into()),
                    name: Some("Arlington".into()),
                }),
                country: Some(PlaceValue {
                    code: Some("US".into()),
                    name: Some("United States".into()),
                }),
                zip: Some("22201".into()),
            }),
        };

        db.upsert_opportunity(&opp).unwrap();

        let title: String = db
            .conn
            .query_row(
                "SELECT title FROM opportunities WHERE notice_id = 'FULL-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(title, "Full Test");

        let awardee: String = db
            .conn
            .query_row(
                "SELECT awardee_name FROM opportunities WHERE notice_id = 'FULL-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(awardee, "Acme");

        let contact_count: i64 = db
            .conn
            .query_row(
                "SELECT COUNT(*) FROM contacts WHERE notice_id = 'FULL-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(contact_count, 1);
    }

    #[test]
    fn test_upsert_minimal_opportunity() {
        let mut db = Database::open_in_memory().unwrap();
        let opp = make_opportunity("MIN-001", "Minimal");
        db.upsert_opportunity(&opp).unwrap();

        let row_count: i64 = db
            .conn
            .query_row(
                "SELECT COUNT(*) FROM opportunities WHERE notice_id = 'MIN-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(row_count, 1);
    }

    #[test]
    fn test_upsert_none_notice_id() {
        let mut db = Database::open_in_memory().unwrap();
        let opp = Opportunity {
            notice_id: None,
            ..make_opportunity("ignored", "ignored")
        };
        db.upsert_opportunity(&opp).unwrap();

        let count: i64 = db
            .conn
            .query_row("SELECT COUNT(*) FROM opportunities", [], |row| row.get(0))
            .unwrap();
        assert_eq!(count, 0);
    }

    #[test]
    fn test_upsert_updates_on_conflict() {
        let mut db = Database::open_in_memory().unwrap();

        let opp1 = make_opportunity("UPD-001", "Original Title");
        db.upsert_opportunity(&opp1).unwrap();

        let opp2 = make_opportunity("UPD-001", "Updated Title");
        db.upsert_opportunity(&opp2).unwrap();

        let title: String = db
            .conn
            .query_row(
                "SELECT title FROM opportunities WHERE notice_id = 'UPD-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(title, "Updated Title");

        let count: i64 = db
            .conn
            .query_row("SELECT COUNT(*) FROM opportunities", [], |row| row.get(0))
            .unwrap();
        assert_eq!(count, 1);
    }

    #[test]
    fn test_upsert_replaces_contacts() {
        let mut db = Database::open_in_memory().unwrap();

        let mut opp = make_opportunity("CON-001", "Contacts Test");
        opp.point_of_contact = Some(vec![
            PointOfContact {
                contact_type: Some("Primary".into()),
                full_name: Some("Alice".into()),
                email: None,
                phone: None,
                title: None,
            },
            PointOfContact {
                contact_type: Some("Secondary".into()),
                full_name: Some("Bob".into()),
                email: None,
                phone: None,
                title: None,
            },
        ]);
        db.upsert_opportunity(&opp).unwrap();

        let count: i64 = db
            .conn
            .query_row(
                "SELECT COUNT(*) FROM contacts WHERE notice_id = 'CON-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 2);

        // Re-upsert with only 1 contact
        opp.point_of_contact = Some(vec![PointOfContact {
            contact_type: Some("Primary".into()),
            full_name: Some("Charlie".into()),
            email: None,
            phone: None,
            title: None,
        }]);
        db.upsert_opportunity(&opp).unwrap();

        let count: i64 = db
            .conn
            .query_row(
                "SELECT COUNT(*) FROM contacts WHERE notice_id = 'CON-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(count, 1);

        let name: String = db
            .conn
            .query_row(
                "SELECT full_name FROM contacts WHERE notice_id = 'CON-001'",
                [],
                |row| row.get(0),
            )
            .unwrap();
        assert_eq!(name, "Charlie");
    }

    #[test]
    fn test_sync_state_get_set() {
        let db = Database::open_in_memory().unwrap();
        assert_eq!(db.get_sync_state("backfill_cursor").unwrap(), None);

        db.set_sync_state("backfill_cursor", "01/01/2025").unwrap();
        assert_eq!(
            db.get_sync_state("backfill_cursor").unwrap(),
            Some("01/01/2025".to_string())
        );

        db.set_sync_state("backfill_cursor", "12/01/2024").unwrap();
        assert_eq!(
            db.get_sync_state("backfill_cursor").unwrap(),
            Some("12/01/2024".to_string())
        );
    }

    #[test]
    fn test_get_earliest_posted_date() {
        let mut db = Database::open_in_memory().unwrap();
        assert_eq!(db.get_earliest_posted_date().unwrap(), None);

        let mut opp1 = make_opportunity("E-001", "First");
        opp1.posted_date = Some("03/15/2025".into());
        db.upsert_opportunity(&opp1).unwrap();

        let mut opp2 = make_opportunity("E-002", "Second");
        opp2.posted_date = Some("01/10/2025".into());
        db.upsert_opportunity(&opp2).unwrap();

        assert_eq!(
            db.get_earliest_posted_date().unwrap(),
            Some("01/10/2025".to_string())
        );
    }

    #[test]
    fn test_log_api_call() {
        let db = Database::open_in_memory().unwrap();
        db.log_api_call(
            "incremental",
            Some("01/01/2025"),
            Some("01/03/2025"),
            1,
            42,
            false,
            None,
        )
        .unwrap();

        let logs = db.get_api_call_logs(10).unwrap();
        assert_eq!(logs.len(), 1);
        assert_eq!(logs[0].context, "incremental");
        assert_eq!(logs[0].posted_from.as_deref(), Some("01/01/2025"));
        assert_eq!(logs[0].posted_to.as_deref(), Some("01/03/2025"));
        assert_eq!(logs[0].api_calls, 1);
        assert_eq!(logs[0].records_fetched, 42);
        assert!(!logs[0].rate_limited);
        assert!(logs[0].error_message.is_none());
    }

    #[test]
    fn test_log_api_call_rate_limited() {
        let db = Database::open_in_memory().unwrap();
        db.log_api_call(
            "backfill",
            Some("01/01/2024"),
            Some("03/31/2024"),
            2,
            0,
            true,
            Some("429 Too Many Requests"),
        )
        .unwrap();

        let logs = db.get_api_call_logs(10).unwrap();
        assert_eq!(logs.len(), 1);
        assert!(logs[0].rate_limited);
        assert_eq!(
            logs[0].error_message.as_deref(),
            Some("429 Too Many Requests")
        );
    }

    #[test]
    fn test_log_api_call_ordering() {
        let db = Database::open_in_memory().unwrap();
        db.log_api_call("incremental", None, None, 1, 10, false, None)
            .unwrap();
        db.log_api_call("backfill", None, None, 1, 20, false, None)
            .unwrap();
        db.log_api_call("incremental", None, None, 1, 30, false, None)
            .unwrap();

        let logs = db.get_api_call_logs(10).unwrap();
        assert_eq!(logs.len(), 3);
        // Most recent first (DESC order)
        assert_eq!(logs[0].records_fetched, 30);
        assert_eq!(logs[1].records_fetched, 20);
        assert_eq!(logs[2].records_fetched, 10);
    }

    #[test]
    fn test_log_api_call_limit() {
        let db = Database::open_in_memory().unwrap();
        for i in 0..5 {
            db.log_api_call("test", None, None, 1, i, false, None)
                .unwrap();
        }

        let logs = db.get_api_call_logs(2).unwrap();
        assert_eq!(logs.len(), 2);
    }

    #[test]
    fn test_log_api_call_pruning() {
        let db = Database::open_in_memory().unwrap();
        for i in 0..205 {
            db.log_api_call("test", None, None, 1, i, false, None)
                .unwrap();
        }

        let logs = db.get_api_call_logs(300).unwrap();
        assert_eq!(logs.len(), 200);
    }

    #[test]
    fn test_upsert_opportunities_batch() {
        let mut db = Database::open_in_memory().unwrap();

        let response = ApiResponse {
            total_records: Some(3),
            opportunities_data: Some(vec![
                make_opportunity("BATCH-1", "First"),
                make_opportunity("BATCH-2", "Second"),
                make_opportunity("BATCH-3", "Third"),
            ]),
        };

        db.upsert_opportunities(&response).unwrap();

        let count: i64 = db
            .conn
            .query_row("SELECT COUNT(*) FROM opportunities", [], |row| row.get(0))
            .unwrap();
        assert_eq!(count, 3);
    }
}
