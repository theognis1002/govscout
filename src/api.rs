use anyhow::{bail, Context, Result};
use reqwest::blocking::Client;
use serde::{Deserialize, Serialize};

const BASE_URL: &str = "https://api.sam.gov/opportunities/v2/search";

pub struct SearchParams {
    pub limit: u32,
    pub offset: u32,
    pub posted_from: String,
    pub posted_to: String,
    pub title: Option<String>,
    pub ptype: Option<String>,
    pub naics: Option<String>,
    pub state: Option<String>,
    pub set_aside: Option<String>,
    pub notice_id: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ApiResponse {
    pub total_records: Option<u64>,
    pub opportunities_data: Option<Vec<Opportunity>>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Opportunity {
    pub notice_id: Option<String>,
    pub title: Option<String>,
    pub solicitation_number: Option<String>,
    pub department: Option<String>,
    pub sub_tier: Option<String>,
    pub office: Option<String>,
    pub full_parent_path_name: Option<String>,
    pub organization_type: Option<String>,
    #[serde(rename = "type")]
    pub opp_type: Option<String>,
    pub base_type: Option<String>,
    pub posted_date: Option<String>,
    pub response_deadline: Option<String>,
    pub archive_date: Option<String>,
    pub naics_code: Option<String>,
    pub classification_code: Option<String>,
    pub set_aside: Option<String>,
    pub set_aside_description: Option<String>,
    pub description: Option<String>,
    pub ui_link: Option<String>,
    pub resource_links: Option<Vec<String>>,
    pub award: Option<Award>,
    pub point_of_contact: Option<Vec<PointOfContact>>,
    pub place_of_performance: Option<PlaceOfPerformance>,
    pub active: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Award {
    pub amount: Option<String>,
    pub date: Option<String>,
    pub number: Option<String>,
    pub awardee: Option<Awardee>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Awardee {
    pub name: Option<String>,
    pub duns: Option<String>,
    pub uei_sam: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PointOfContact {
    #[serde(rename = "type")]
    pub contact_type: Option<String>,
    pub full_name: Option<String>,
    pub email: Option<String>,
    pub phone: Option<String>,
    pub title: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PlaceOfPerformance {
    pub state: Option<PlaceValue>,
    pub city: Option<PlaceValue>,
    pub country: Option<PlaceValue>,
    pub zip: Option<String>,
}

#[derive(Debug, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PlaceValue {
    pub code: Option<String>,
    pub name: Option<String>,
}

pub struct SamGovClient {
    client: Client,
    api_key: String,
}

impl SamGovClient {
    pub fn new() -> Result<Self> {
        let api_key = std::env::var("SAMGOV_API_KEY")
            .context("SAMGOV_API_KEY not found. Set it in .env or as an environment variable.")?;

        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()
            .context("Failed to build HTTP client")?;

        Ok(Self { client, api_key })
    }

    pub fn search(&self, params: &SearchParams) -> Result<ApiResponse> {
        let mut query: Vec<(&str, String)> = vec![
            ("api_key", self.api_key.clone()),
            ("limit", params.limit.to_string()),
            ("offset", params.offset.to_string()),
        ];

        // Date range is only required when not searching by notice ID
        if params.notice_id.is_none() {
            query.push(("postedFrom", params.posted_from.clone()));
            query.push(("postedTo", params.posted_to.clone()));
        }

        if let Some(ref title) = params.title {
            query.push(("title", title.clone()));
        }
        if let Some(ref ptype) = params.ptype {
            query.push(("ptype", ptype.clone()));
        }
        if let Some(ref naics) = params.naics {
            query.push(("ncode", naics.clone()));
        }
        if let Some(ref state) = params.state {
            query.push(("state", state.clone()));
        }
        if let Some(ref set_aside) = params.set_aside {
            query.push(("typeOfSetAside", set_aside.clone()));
        }
        if let Some(ref notice_id) = params.notice_id {
            query.push(("noticeid", notice_id.clone()));
        }

        let response = self
            .client
            .get(BASE_URL)
            .query(&query)
            .send()
            .context("Failed to connect to SAM.gov API")?;

        let status = response.status();
        if !status.is_success() {
            let body = response.text().unwrap_or_default();
            bail!("SAM.gov API returned {status}: {body}");
        }

        let api_response: ApiResponse = response
            .json()
            .context("Failed to parse SAM.gov API response")?;

        Ok(api_response)
    }

    pub fn get(&self, notice_id: &str) -> Result<Opportunity> {
        let params = SearchParams {
            limit: 1,
            offset: 0,
            posted_from: String::new(),
            posted_to: String::new(),
            title: None,
            ptype: None,
            naics: None,
            state: None,
            set_aside: None,
            notice_id: Some(notice_id.to_string()),
        };

        let response = self.search(&params)?;

        match response.opportunities_data {
            Some(mut opps) if !opps.is_empty() => Ok(opps.remove(0)),
            _ => bail!("No opportunity found with notice ID: {notice_id}"),
        }
    }
}
