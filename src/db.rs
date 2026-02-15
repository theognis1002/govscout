use anyhow::{Context, Result};
use rusqlite::Connection;
use std::path::PathBuf;

use crate::api::{ApiResponse, Opportunity};

pub struct Database {
    conn: Connection,
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

        conn.execute_batch(
            "PRAGMA journal_mode=WAL;
             PRAGMA synchronous=NORMAL;
             PRAGMA foreign_keys=ON;
             PRAGMA busy_timeout=5000;",
        )
        .context("Failed to set database pragmas")?;

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
                CREATE INDEX IF NOT EXISTS idx_contacts_notice ON contacts(notice_id);",
            )
            .context("Failed to initialize database schema")?;

        Ok(())
    }

    pub fn upsert_opportunity(&self, opp: &Opportunity) -> Result<()> {
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

        self.conn
            .execute(
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
        self.conn
            .execute(
                "DELETE FROM contacts WHERE notice_id = ?1",
                rusqlite::params![notice_id],
            )
            .context("Failed to delete existing contacts")?;

        if let Some(contacts) = &opp.point_of_contact {
            let mut stmt = self.conn.prepare(
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

    pub fn upsert_opportunities(&self, response: &ApiResponse) -> Result<()> {
        let opps = match &response.opportunities_data {
            Some(opps) => opps,
            None => return Ok(()),
        };

        let tx = self.conn.unchecked_transaction()?;
        for opp in opps {
            self.upsert_opportunity(opp)?;
        }
        tx.commit().context("Failed to commit transaction")?;

        Ok(())
    }
}

fn resolve_db_path() -> Result<PathBuf> {
    if let Ok(path) = std::env::var("GOVSCOUT_DB") {
        return Ok(PathBuf::from(path));
    }

    if let Some(data_dir) = dirs::data_dir() {
        return Ok(data_dir.join("govscout").join("govscout.db"));
    }

    Ok(PathBuf::from("govscout.db"))
}
