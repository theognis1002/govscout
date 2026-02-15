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
            .user_agent(format!("govscout/{}", env!("CARGO_PKG_VERSION")))
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
            .map_err(|e| {
                let msg = e.to_string().replace(&self.api_key, "[REDACTED]");
                anyhow::anyhow!("Failed to connect to SAM.gov API: {msg}")
            })?;

        let status = response.status();
        if !status.is_success() {
            let body = response
                .text()
                .unwrap_or_default()
                .replace(&self.api_key, "[REDACTED]");
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_deserialize_full_opportunity() {
        let json = r#"{
            "noticeId": "ABC123",
            "title": "Cloud Services",
            "solicitationNumber": "SOL-001",
            "department": "DOD",
            "subTier": "Army",
            "office": "ACC",
            "fullParentPathName": "DOD.Army.ACC",
            "organizationType": "DEPT/AGENCY",
            "type": "Solicitation",
            "baseType": "Presolicitation",
            "postedDate": "01/15/2026",
            "responseDeadline": "02/15/2026",
            "archiveDate": "03/15/2026",
            "naicsCode": "541512",
            "classificationCode": "D302",
            "setAside": "SBA",
            "setAsideDescription": "Total Small Business",
            "description": "<p>Need cloud</p>",
            "uiLink": "https://sam.gov/opp/abc",
            "active": "Yes",
            "resourceLinks": ["https://example.com/doc.pdf"],
            "award": {
                "amount": "1000000",
                "date": "2026-01-01",
                "number": "AWD-001",
                "awardee": {
                    "name": "Acme Corp",
                    "duns": "123456789",
                    "ueiSAM": "UEI123"
                }
            },
            "pointOfContact": [
                {
                    "type": "Primary",
                    "fullName": "Jane Doe",
                    "email": "jane@gov.gov",
                    "phone": "555-1234",
                    "title": "Contracting Officer"
                }
            ],
            "placeOfPerformance": {
                "state": {"code": "VA", "name": "Virginia"},
                "city": {"code": "001", "name": "Arlington"},
                "country": {"code": "US", "name": "United States"},
                "zip": "22201"
            }
        }"#;

        let opp: Opportunity = serde_json::from_str(json).unwrap();
        assert_eq!(opp.notice_id.as_deref(), Some("ABC123"));
        assert_eq!(opp.title.as_deref(), Some("Cloud Services"));
        assert_eq!(opp.opp_type.as_deref(), Some("Solicitation"));
        assert_eq!(opp.award.as_ref().unwrap().awardee.as_ref().unwrap().name.as_deref(), Some("Acme Corp"));
        assert_eq!(opp.place_of_performance.as_ref().unwrap().state.as_ref().unwrap().code.as_deref(), Some("VA"));
        assert_eq!(opp.point_of_contact.as_ref().unwrap()[0].contact_type.as_deref(), Some("Primary"));
    }

    #[test]
    fn test_deserialize_minimal_opportunity() {
        let json = "{}";
        let opp: Opportunity = serde_json::from_str(json).unwrap();
        assert!(opp.notice_id.is_none());
        assert!(opp.title.is_none());
        assert!(opp.award.is_none());
        assert!(opp.point_of_contact.is_none());
        assert!(opp.place_of_performance.is_none());
    }

    #[test]
    fn test_deserialize_api_response() {
        let json = r#"{
            "totalRecords": 2,
            "opportunitiesData": [
                {"noticeId": "A1", "title": "First"},
                {"noticeId": "A2", "title": "Second"}
            ]
        }"#;

        let resp: ApiResponse = serde_json::from_str(json).unwrap();
        assert_eq!(resp.total_records, Some(2));
        let opps = resp.opportunities_data.unwrap();
        assert_eq!(opps.len(), 2);
        assert_eq!(opps[0].notice_id.as_deref(), Some("A1"));
        assert_eq!(opps[1].title.as_deref(), Some("Second"));
    }

    #[test]
    fn test_serialize_roundtrip() {
        let opp = Opportunity {
            notice_id: Some("RT-001".into()),
            title: Some("Roundtrip Test".into()),
            solicitation_number: None,
            department: Some("DOE".into()),
            sub_tier: None,
            office: None,
            full_parent_path_name: None,
            organization_type: None,
            opp_type: Some("Solicitation".into()),
            base_type: None,
            posted_date: Some("01/01/2026".into()),
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
        };

        let json = serde_json::to_string(&opp).unwrap();
        let deserialized: Opportunity = serde_json::from_str(&json).unwrap();
        assert_eq!(deserialized.notice_id, opp.notice_id);
        assert_eq!(deserialized.title, opp.title);
        assert_eq!(deserialized.opp_type, opp.opp_type);
        assert_eq!(deserialized.posted_date, opp.posted_date);
    }
}
