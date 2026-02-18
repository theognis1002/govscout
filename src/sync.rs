use anyhow::Result;
use chrono::{Local, NaiveDate};

use crate::api::SamGovClient;
use crate::db::Database;

const BACKFILL_WINDOW_DAYS: i64 = 90;
const INCREMENTAL_DAYS: i64 = 3;
const DATE_FMT: &str = "%m/%d/%Y";

pub struct SyncSummary {
    pub api_calls_used: u32,
    pub records_synced: usize,
    pub windows_completed: u32,
    pub rate_limited: bool,
    pub backfill_cursor: Option<String>,
}

pub fn run_sync(
    max_api_calls: u32,
    dry_run: bool,
    from_override: Option<&str>,
) -> Result<SyncSummary> {
    let client = SamGovClient::new()?;
    let mut db = Database::open()?;

    let today = Local::now().date_naive();
    let mut api_calls_used: u32 = 0;
    let mut records_synced: usize = 0;
    let mut windows_completed: u32 = 0;
    let mut rate_limited = false;

    // Phase 1: Incremental sync (last INCREMENTAL_DAYS days)
    let incr_from = (today - chrono::Duration::days(INCREMENTAL_DAYS))
        .format(DATE_FMT)
        .to_string();
    let incr_to = today.format(DATE_FMT).to_string();

    eprintln!("Incremental sync: {} to {}", incr_from, incr_to);

    if dry_run {
        eprintln!("  [dry-run] Would fetch window {} - {}", incr_from, incr_to);
    } else {
        let result = client.search_window(&incr_from, &incr_to, &mut |page| {
            if let Err(e) = db.upsert_opportunities(page) {
                eprintln!("DB upsert error: {e}");
            }
        })?;

        if let Err(e) = db.log_api_call(
            "incremental",
            Some(&incr_from),
            Some(&incr_to),
            result.api_calls,
            result.records_fetched,
            result.rate_limited,
            None,
        ) {
            eprintln!("Failed to log API call: {e}");
        }

        api_calls_used += result.api_calls;
        records_synced += result.records_fetched;
        windows_completed += 1;

        eprintln!(
            "  Fetched {} records ({} API call{})",
            result.records_fetched,
            result.api_calls,
            if result.api_calls == 1 { "" } else { "s" }
        );

        if result.rate_limited {
            rate_limited = true;
            eprintln!("  Rate limited during incremental sync, stopping.");
            db.set_sync_state("last_sync", &today.format(DATE_FMT).to_string())?;
            return Ok(SyncSummary {
                api_calls_used,
                records_synced,
                windows_completed,
                rate_limited,
                backfill_cursor: db.get_sync_state("backfill_cursor")?,
            });
        }
    }

    // Phase 2: Backfill with remaining budget
    let remaining = max_api_calls.saturating_sub(api_calls_used);
    if remaining < 2 {
        eprintln!("No API budget remaining for backfill.");
    } else {
        eprintln!("Backfill: {} API calls remaining", remaining);

        // Determine backfill cursor
        let mut cursor = if let Some(from_str) = from_override {
            eprintln!("  Using --from override: {}", from_str);
            // --from specifies the target start date; we backfill backwards from today toward it
            // So cursor starts at today minus incremental window (to avoid overlap with phase 1)
            today - chrono::Duration::days(INCREMENTAL_DAYS)
        } else {
            let cursor_str = db.get_sync_state("backfill_cursor")?;
            match cursor_str {
                Some(ref s) => parse_date(s)?,
                None => match db.get_earliest_posted_date()? {
                    Some(ref s) => parse_date(s)?,
                    None => today - chrono::Duration::days(INCREMENTAL_DAYS),
                },
            }
        };

        // If --from is provided, stop backfilling once we reach that date
        let backfill_floor = from_override.map(parse_date).transpose()?;

        while api_calls_used + 2 <= max_api_calls {
            if let Some(floor) = backfill_floor {
                if cursor <= floor {
                    eprintln!(
                        "  Reached --from date {}, stopping backfill.",
                        floor.format(DATE_FMT)
                    );
                    break;
                }
            }

            let window_to = cursor;
            let window_from = cursor - chrono::Duration::days(BACKFILL_WINDOW_DAYS);

            let from_str = window_from.format(DATE_FMT).to_string();
            let to_str = window_to.format(DATE_FMT).to_string();

            eprintln!("  Backfill window: {} to {}", from_str, to_str);

            if dry_run {
                eprintln!("    [dry-run] Would fetch this window");
                windows_completed += 1;
                api_calls_used += 1; // estimate 1 call per window for budget tracking
                cursor = window_from;
                continue;
            }

            let result = client.search_window(&from_str, &to_str, &mut |page| {
                if let Err(e) = db.upsert_opportunities(page) {
                    eprintln!("DB upsert error: {e}");
                }
            })?;

            if let Err(e) = db.log_api_call(
                "backfill",
                Some(&from_str),
                Some(&to_str),
                result.api_calls,
                result.records_fetched,
                result.rate_limited,
                None,
            ) {
                eprintln!("Failed to log API call: {e}");
            }

            api_calls_used += result.api_calls;
            records_synced += result.records_fetched;
            windows_completed += 1;

            eprintln!(
                "    {} records ({} API call{})",
                result.records_fetched,
                result.api_calls,
                if result.api_calls == 1 { "" } else { "s" }
            );

            cursor = window_from;
            db.set_sync_state("backfill_cursor", &cursor.format(DATE_FMT).to_string())?;

            if result.rate_limited {
                rate_limited = true;
                eprintln!("  Rate limited, stopping backfill.");
                break;
            }
        }
    }

    if !dry_run {
        db.set_sync_state("last_sync", &today.format(DATE_FMT).to_string())?;
    }

    let final_cursor = db.get_sync_state("backfill_cursor")?;

    Ok(SyncSummary {
        api_calls_used,
        records_synced,
        windows_completed,
        rate_limited,
        backfill_cursor: final_cursor,
    })
}

pub(crate) fn parse_date(s: &str) -> Result<NaiveDate> {
    NaiveDate::parse_from_str(s, DATE_FMT)
        .map_err(|e| anyhow::anyhow!("Failed to parse date '{}': {}", s, e))
}

pub fn print_summary(summary: &SyncSummary) {
    eprintln!();
    eprintln!("=== Sync Summary ===");
    eprintln!("  API calls used:     {}", summary.api_calls_used);
    eprintln!("  Records synced:     {}", summary.records_synced);
    eprintln!("  Windows completed:  {}", summary.windows_completed);
    if let Some(ref cursor) = summary.backfill_cursor {
        eprintln!("  Backfill cursor:    {}", cursor);
    }
    if summary.rate_limited {
        eprintln!("  Status:             Rate limited (will resume next run)");
    } else {
        eprintln!("  Status:             Complete");
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_date_valid() {
        let d = parse_date("01/15/2025").unwrap();
        assert_eq!(d, NaiveDate::from_ymd_opt(2025, 1, 15).unwrap());
    }

    #[test]
    fn test_parse_date_various_formats() {
        assert!(parse_date("12/31/2024").is_ok());
        assert!(parse_date("02/28/2025").is_ok());
    }

    #[test]
    fn test_parse_date_invalid() {
        assert!(parse_date("2025-01-15").is_err());
        assert!(parse_date("not-a-date").is_err());
        assert!(parse_date("").is_err());
        assert!(parse_date("13/01/2025").is_err());
    }

    #[test]
    fn test_sync_summary_defaults() {
        let summary = SyncSummary {
            api_calls_used: 0,
            records_synced: 0,
            windows_completed: 0,
            rate_limited: false,
            backfill_cursor: None,
        };
        assert_eq!(summary.api_calls_used, 0);
        assert!(!summary.rate_limited);
        assert!(summary.backfill_cursor.is_none());
    }

    #[test]
    fn test_sync_summary_with_cursor() {
        let summary = SyncSummary {
            api_calls_used: 5,
            records_synced: 1200,
            windows_completed: 3,
            rate_limited: true,
            backfill_cursor: Some("06/15/2023".to_string()),
        };
        assert_eq!(summary.api_calls_used, 5);
        assert_eq!(summary.records_synced, 1200);
        assert!(summary.rate_limited);
        assert_eq!(summary.backfill_cursor.as_deref(), Some("06/15/2023"));
    }

    #[test]
    fn test_constants() {
        assert_eq!(BACKFILL_WINDOW_DAYS, 90);
        assert_eq!(INCREMENTAL_DAYS, 3);
        assert_eq!(DATE_FMT, "%m/%d/%Y");
    }
}
