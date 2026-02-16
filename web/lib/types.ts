export interface OpportunityRow {
  notice_id: string | null;
  title: string | null;
  solicitation_number: string | null;
  department: string | null;
  sub_tier: string | null;
  office: string | null;
  opp_type: string | null;
  base_type: string | null;
  posted_date: string | null;
  response_deadline: string | null;
  naics_code: string | null;
  set_aside: string | null;
  set_aside_description: string | null;
  active: string | null;
  ui_link: string | null;
  pop_state_code: string | null;
  pop_state_name: string | null;
}

export interface ListResponse {
  total: number;
  limit: number;
  offset: number;
  opportunities: OpportunityRow[];
}

export interface OpportunityDetail {
  notice_id: string | null;
  title: string | null;
  solicitation_number: string | null;
  department: string | null;
  sub_tier: string | null;
  office: string | null;
  full_parent_path_name: string | null;
  organization_type: string | null;
  opp_type: string | null;
  base_type: string | null;
  posted_date: string | null;
  response_deadline: string | null;
  archive_date: string | null;
  naics_code: string | null;
  classification_code: string | null;
  set_aside: string | null;
  set_aside_description: string | null;
  description: string | null;
  ui_link: string | null;
  active: string | null;
  resource_links: string[] | null;
  award_amount: string | null;
  award_date: string | null;
  award_number: string | null;
  awardee_name: string | null;
  awardee_uei_sam: string | null;
  pop_state_code: string | null;
  pop_state_name: string | null;
  pop_city_name: string | null;
  pop_country_name: string | null;
  pop_zip: string | null;
}

export interface ContactRow {
  contact_type: string | null;
  full_name: string | null;
  email: string | null;
  phone: string | null;
  title: string | null;
}

export interface DetailResponse {
  opportunity: OpportunityDetail;
  contacts: ContactRow[];
}

export interface FilterOption {
  value: string;
  count: number;
}

export interface StatsResponse {
  total_opportunities: number;
  naics_codes: FilterOption[];
  opp_types: FilterOption[];
  set_asides: FilterOption[];
  states: FilterOption[];
  departments: FilterOption[];
}

export interface SearchFiltersState {
  search?: string;
  naics_code?: string;
  opp_type?: string;
  set_aside?: string;
  state?: string;
  department?: string;
  active_only?: boolean;
  limit?: number;
  offset?: number;
}
