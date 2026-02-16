use crate::api::{ApiResponse, Opportunity};
use tabled::Tabled;
use tabled::{settings::Style, Table};

#[derive(Tabled)]
struct SearchRow {
    #[tabled(rename = "Notice ID")]
    notice_id: String,
    #[tabled(rename = "Title")]
    title: String,
    #[tabled(rename = "Type")]
    opp_type: String,
    #[tabled(rename = "Posted")]
    posted: String,
    #[tabled(rename = "Organization")]
    org: String,
}

pub fn print_search_results(response: &ApiResponse) {
    let total = response.total_records.unwrap_or(0);
    let opps = match &response.opportunities_data {
        Some(opps) if !opps.is_empty() => opps,
        _ => {
            println!("No opportunities found.");
            return;
        }
    };

    println!("Showing {} of {} results\n", opps.len(), total);

    let rows: Vec<SearchRow> = opps
        .iter()
        .map(|opp| SearchRow {
            notice_id: opp.notice_id.as_deref().unwrap_or("—").to_string(),
            title: truncate(opp.title.as_deref().unwrap_or("—"), 50),
            opp_type: opp.base_type.as_deref().unwrap_or("—").to_string(),
            posted: opp.posted_date.as_deref().unwrap_or("—").to_string(),
            org: truncate(
                opp.full_parent_path_name
                    .as_deref()
                    .or(opp.department.as_deref())
                    .or(opp.sub_tier.as_deref())
                    .unwrap_or("—"),
                40,
            ),
        })
        .collect();

    let table = Table::new(rows).with(Style::rounded()).to_string();
    println!("{table}");
}

pub fn print_search_results_paginated(response: &ApiResponse, total_saved: usize) {
    let total = response.total_records.unwrap_or(0);
    let opps = match &response.opportunities_data {
        Some(opps) if !opps.is_empty() => opps,
        _ => {
            println!("No opportunities found.");
            return;
        }
    };

    println!(
        "Showing first {} of {} total results ({} saved to database)\n",
        opps.len(),
        total,
        total_saved,
    );

    let rows: Vec<SearchRow> = opps
        .iter()
        .map(|opp| SearchRow {
            notice_id: opp.notice_id.as_deref().unwrap_or("—").to_string(),
            title: truncate(opp.title.as_deref().unwrap_or("—"), 50),
            opp_type: opp.base_type.as_deref().unwrap_or("—").to_string(),
            posted: opp.posted_date.as_deref().unwrap_or("—").to_string(),
            org: truncate(
                opp.full_parent_path_name
                    .as_deref()
                    .or(opp.department.as_deref())
                    .or(opp.sub_tier.as_deref())
                    .unwrap_or("—"),
                40,
            ),
        })
        .collect();

    let table = Table::new(rows).with(Style::rounded()).to_string();
    println!("{table}");
}

pub fn print_opportunity_detail(opp: &Opportunity) {
    let field = |label: &str, value: Option<&str>| {
        if let Some(v) = value {
            println!("  {:<22} {}", label, v);
        }
    };

    println!();
    println!("  ═══ {} ═══", opp.title.as_deref().unwrap_or("Untitled"));
    println!();

    field("Notice ID:", opp.notice_id.as_deref());
    field("Solicitation #:", opp.solicitation_number.as_deref());
    field("Type:", opp.opp_type.as_deref());
    field("Base Type:", opp.base_type.as_deref());
    field("Active:", opp.active.as_deref());
    println!();

    println!("  ── Organization ──");
    field("Organization:", opp.full_parent_path_name.as_deref());
    field("Department:", opp.department.as_deref());
    field("Sub-tier:", opp.sub_tier.as_deref());
    field("Office:", opp.office.as_deref());
    println!();

    println!("  ── Dates ──");
    field("Posted:", opp.posted_date.as_deref());
    field("Response Deadline:", opp.response_deadline.as_deref());
    field("Archive Date:", opp.archive_date.as_deref());
    println!();

    println!("  ── Classification ──");
    field("NAICS Code:", opp.naics_code.as_deref());
    field("Classification Code:", opp.classification_code.as_deref());
    field("Set-Aside:", opp.set_aside.as_deref());
    field("Set-Aside Desc:", opp.set_aside_description.as_deref());
    println!();

    if let Some(ref pop) = opp.place_of_performance {
        println!("  ── Place of Performance ──");
        if let Some(ref city) = pop.city {
            field("City:", city.name.as_deref());
        }
        if let Some(ref state) = pop.state {
            field("State:", state.name.as_deref());
        }
        if let Some(ref country) = pop.country {
            field("Country:", country.name.as_deref());
        }
        field("ZIP:", pop.zip.as_deref());
        println!();
    }

    if let Some(ref contacts) = opp.point_of_contact {
        if !contacts.is_empty() {
            println!("  ── Point(s) of Contact ──");
            for poc in contacts {
                if let Some(ref name) = poc.full_name {
                    println!("    {} {}", poc.contact_type.as_deref().unwrap_or(""), name);
                }
                if let Some(ref email) = poc.email {
                    println!("      Email: {email}");
                }
                if let Some(ref phone) = poc.phone {
                    println!("      Phone: {phone}");
                }
            }
            println!();
        }
    }

    if let Some(ref award) = opp.award {
        println!("  ── Award ──");
        field("Amount:", award.amount.as_deref());
        field("Date:", award.date.as_deref());
        field("Number:", award.number.as_deref());
        if let Some(ref awardee) = award.awardee {
            field("Awardee:", awardee.name.as_deref());
            field("UEI:", awardee.uei_sam.as_deref());
        }
        println!();
    }

    if let Some(ref link) = opp.ui_link {
        println!("  ── Links ──");
        println!("  SAM.gov:  {link}");
    }
    if let Some(ref links) = opp.resource_links {
        for l in links {
            println!("  Resource: {l}");
        }
    }

    if let Some(ref desc) = opp.description {
        println!();
        println!("  ── Description ──");
        // Strip HTML tags for readability
        let clean = strip_html(desc);
        let lines: Vec<&str> = clean.lines().collect();
        for line in lines.iter().take(30) {
            println!("  {line}");
        }
        if lines.len() > 30 {
            println!("  ... (truncated)");
        }
    }

    println!();
}

pub fn print_types() {
    println!("Opportunity Type Codes (--ptype):");
    println!();
    let type_rows = vec![
        ("o", "Solicitation"),
        ("p", "Presolicitation"),
        ("k", "Combined Synopsis/Solicitation"),
        ("r", "Sources Sought"),
        ("s", "Special Notice"),
        ("a", "Award Notice"),
        ("u", "Justification and Approval (J&A)"),
        ("g", "Intent to Bundle"),
        ("i", "Fair Opportunity / Limited Sources Justification"),
    ];
    for (code, desc) in &type_rows {
        println!("  {code:<4} {desc}");
    }

    println!();
    println!("Set-Aside Codes (--set-aside):");
    println!();
    let set_aside_rows = vec![
        ("SBA", "Total Small Business Set-Aside (FAR 19.5)"),
        ("SBP", "Partial Small Business Set-Aside (FAR 19.5)"),
        ("8A", "8(a) Set-Aside (FAR 19.8)"),
        ("8AN", "8(a) Sole Source (FAR 19.8)"),
        ("HZC", "HUBZone Set-Aside (FAR 19.13)"),
        ("HZS", "HUBZone Sole Source (FAR 19.13)"),
        ("SDVOSBC", "SDVOSB Set-Aside (FAR 19.14)"),
        ("SDVOSBS", "SDVOSB Sole Source (FAR 19.14)"),
        ("WOSB", "WOSB Set-Aside (FAR 19.15)"),
        ("WOSBSS", "WOSB Sole Source (FAR 19.15)"),
        ("EDWOSB", "EDWOSB Set-Aside (FAR 19.15)"),
        ("EDWOSBSS", "EDWOSB Sole Source (FAR 19.15)"),
        ("VSA", "Veteran-Owned Small Business Set-Aside"),
        ("VSS", "Veteran-Owned Small Business Sole Source"),
    ];
    for (code, desc) in &set_aside_rows {
        println!("  {code:<12} {desc}");
    }
}

fn truncate(s: &str, max: usize) -> String {
    if s.chars().count() <= max {
        s.to_string()
    } else {
        let end = s
            .char_indices()
            .nth(max - 1)
            .map(|(i, _)| i)
            .unwrap_or(s.len());
        format!("{}…", &s[..end])
    }
}

fn strip_html(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut in_tag = false;
    for c in s.chars() {
        match c {
            '<' => in_tag = true,
            '>' => in_tag = false,
            _ if !in_tag => result.push(c),
            _ => {}
        }
    }
    // Decode common HTML entities
    let result = result
        .replace("&amp;", "&")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&nbsp;", " ")
        .replace("&#39;", "'")
        .replace("&quot;", "\"");
    // Collapse excessive blank lines
    let mut prev_blank = false;
    let mut cleaned = String::new();
    for line in result.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            if !prev_blank {
                cleaned.push('\n');
            }
            prev_blank = true;
        } else {
            cleaned.push_str(trimmed);
            cleaned.push('\n');
            prev_blank = false;
        }
    }
    cleaned
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_truncate_short_string() {
        assert_eq!(truncate("hello", 10), "hello");
    }

    #[test]
    fn test_truncate_exact_length() {
        assert_eq!(truncate("hello", 5), "hello");
    }

    #[test]
    fn test_truncate_long_string() {
        let result = truncate("hello world", 5);
        assert_eq!(result, "hell…");
    }

    #[test]
    fn test_truncate_empty() {
        assert_eq!(truncate("", 10), "");
    }

    #[test]
    fn test_strip_html_tags() {
        let input = "<p>Hello <b>world</b></p>";
        let result = strip_html(input);
        assert_eq!(result.trim(), "Hello world");
    }

    #[test]
    fn test_strip_html_entities() {
        let input = "&amp; &lt; &gt; &nbsp; &#39; &quot;";
        let result = strip_html(input);
        assert_eq!(result.trim(), "& < >   ' \"");
    }

    #[test]
    fn test_strip_html_collapses_blank_lines() {
        let input = "line1\n\n\n\nline2";
        let result = strip_html(input);
        assert_eq!(result, "line1\n\nline2\n");
    }
}
