use anyhow::Result;
use chrono::Local;
use clap::{Parser, Subcommand};

use govscout_lib::api::{SamGovClient, SearchParams};
use govscout_lib::db::Database;
use govscout_lib::display;

/// GovScout — Search and view federal contract opportunities from SAM.gov
#[derive(Parser)]
#[command(name = "govscout", version, about)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Search for contract opportunities
    Search {
        /// Max results to fetch (omit to auto-paginate all results)
        #[arg(short, long, value_parser = clap::value_parser!(u32).range(1..=1000))]
        limit: Option<u32>,

        /// Filter by title keyword
        #[arg(short, long)]
        title: Option<String>,

        /// Opportunity type code (o,p,k,r,s,a,u,g,i)
        #[arg(short, long)]
        ptype: Option<String>,

        /// NAICS code
        #[arg(short, long)]
        naics: Option<String>,

        /// State code (e.g. CA)
        #[arg(short, long)]
        state: Option<String>,

        /// Set-aside type code
        #[arg(long)]
        set_aside: Option<String>,

        /// Posted from date (MM/DD/YYYY)
        #[arg(long)]
        from: Option<String>,

        /// Posted to date (MM/DD/YYYY)
        #[arg(long)]
        to: Option<String>,

        /// Output raw JSON
        #[arg(long)]
        json: bool,
    },

    /// View a specific opportunity by notice ID
    Get {
        /// The notice ID to look up
        notice_id: String,

        /// Output raw JSON
        #[arg(long)]
        json: bool,
    },

    /// Print opportunity type and set-aside reference codes
    Types,

    /// Sync opportunities: incremental update + historical backfill
    Sync {
        /// Max API calls for this run
        #[arg(long, default_value = "18")]
        max_calls: u32,

        /// Show what would be fetched without making API calls
        #[arg(long)]
        dry_run: bool,

        /// Override backfill start date (MM/DD/YYYY) — backfill from today toward this date
        #[arg(long)]
        from: Option<String>,
    },
}

fn main() -> Result<()> {
    dotenvy::dotenv().ok();
    let cli = Cli::parse();

    match cli.command {
        Commands::Search {
            limit,
            title,
            ptype,
            naics,
            state,
            set_aside,
            from,
            to,
            json,
        } => {
            let now = Local::now();
            let default_from = (now - chrono::Duration::days(30))
                .format("%m/%d/%Y")
                .to_string();
            let default_to = now.format("%m/%d/%Y").to_string();

            let params = SearchParams {
                limit: limit.unwrap_or(1000),
                offset: 0,
                posted_from: from.unwrap_or(default_from),
                posted_to: to.unwrap_or(default_to),
                title,
                ptype,
                naics,
                state,
                set_aside,
                notice_id: None,
            };

            let client = SamGovClient::new()?;
            let mut db = Database::open()?;

            if let Some(_limit) = limit {
                // Single-page fetch with explicit limit
                let response = client.search(&params)?;
                db.upsert_opportunities(&response)?;
                if json {
                    println!("{}", serde_json::to_string_pretty(&response)?);
                } else {
                    display::print_search_results(&response);
                }
            } else {
                // Auto-paginate all results
                let (first_page, total_saved) = client.search_all(&params, |page| {
                    db.upsert_opportunities(page).ok();
                })?;
                if json {
                    println!("{}", serde_json::to_string_pretty(&first_page)?);
                } else {
                    display::print_search_results_paginated(&first_page, total_saved);
                }
            }
        }

        Commands::Get { notice_id, json } => {
            let client = SamGovClient::new()?;
            let opp = client.get(&notice_id)?;
            let mut db = Database::open()?;
            db.upsert_opportunity(&opp)?;
            if json {
                println!("{}", serde_json::to_string_pretty(&opp)?);
            } else {
                display::print_opportunity_detail(&opp);
            }
        }

        Commands::Types => {
            display::print_types();
        }

        Commands::Sync { max_calls, dry_run, from } => {
            let summary = govscout_lib::sync::run_sync(max_calls, dry_run, from.as_deref())?;
            govscout_lib::sync::print_summary(&summary);
        }
    }

    Ok(())
}
