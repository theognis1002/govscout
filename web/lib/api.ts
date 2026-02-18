import type { ListResponse, DetailResponse, StatsResponse, SearchFiltersState, ApiCallLogEntry } from "./types";

function getApiBase() {
  return process.env.API_URL || "http://localhost:3001";
}

export async function fetchOpportunities(
  filters: SearchFiltersState = {}
): Promise<ListResponse> {
  const params = new URLSearchParams();

  if (filters.search) params.set("search", filters.search);
  if (filters.naics_code) params.set("naics_code", filters.naics_code);
  if (filters.opp_type) params.set("opp_type", filters.opp_type);
  if (filters.set_aside) params.set("set_aside", filters.set_aside);
  if (filters.state) params.set("state", filters.state);
  if (filters.department) params.set("department", filters.department);
  if (filters.active_only) params.set("active_only", "true");
  if (filters.limit) params.set("limit", filters.limit.toString());
  if (filters.offset !== undefined) params.set("offset", filters.offset.toString());

  const qs = params.toString();
  const res = await fetch(`${getApiBase()}/api/opportunities${qs ? `?${qs}` : ""}`, {
    cache: "no-store",
  });

  if (!res.ok) throw new Error(`Failed to fetch opportunities: ${res.status}`);
  return res.json();
}

export async function fetchOpportunity(id: string): Promise<DetailResponse> {
  const res = await fetch(`${getApiBase()}/api/opportunities/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });

  if (!res.ok) throw new Error(`Failed to fetch opportunity: ${res.status}`);
  return res.json();
}

export async function fetchStats(): Promise<StatsResponse> {
  const res = await fetch(`${getApiBase()}/api/stats`, {
    cache: "no-store",
  });

  if (!res.ok) throw new Error(`Failed to fetch stats: ${res.status}`);
  return res.json();
}

export async function fetchApiCallLogs(limit: number = 100): Promise<ApiCallLogEntry[]> {
  const res = await fetch(`${getApiBase()}/api/api-calls?limit=${limit}`, {
    cache: "no-store",
  });

  if (!res.ok) throw new Error(`Failed to fetch API call logs: ${res.status}`);
  return res.json();
}
