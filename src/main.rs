mod api;
mod db;
mod display;

use anyhow::Result;
use chrono::Local;
use clap::{Parser, Subcommand};

use api::{SamGovClient, SearchParams};
use db::Database;

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
        /// Number of results (1-1000)
        #[arg(short, long, default_value_t = 10, value_parser = clap::value_parser!(u32).range(1..=1000))]
        limit: u32,

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

        /// Pagination offset
        #[arg(long, default_value_t = 0)]
        offset: u32,

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
            offset,
            json,
        } => {
            let now = Local::now();
            let default_from = (now - chrono::Duration::days(30))
                .format("%m/%d/%Y")
                .to_string();
            let default_to = now.format("%m/%d/%Y").to_string();

            let params = SearchParams {
                limit,
                offset,
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
            let response = client.search(&params)?;
            let mut db = Database::open()?;
            db.upsert_opportunities(&response)?;
            if json {
                println!("{}", serde_json::to_string_pretty(&response)?);
            } else {
                display::print_search_results(&response);
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
    }

    Ok(())
}
