"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { StatsResponse } from "@/lib/types";
import { NAICS_LABELS } from "@/lib/naics-labels";

const ALL_VALUE = "__all__";

const SET_ASIDE_LABELS: Record<string, string> = {
  SBA: "Total Small Business Set-Aside (FAR 19.5)",
  SBP: "Partial Small Business Set-Aside (FAR 19.5)",
  "8A": "8(a) Set-Aside (FAR 19.8)",
  "8AN": "8(a) Sole Source (FAR 19.8)",
  HZC: "HUBZone Set-Aside (FAR 19.13)",
  HZS: "HUBZone Sole Source (FAR 19.13)",
  SDVOSBC: "Service-Disabled Veteran-Owned SB Set-Aside",
  SDVOSBS: "SDVOSB Sole Source",
  WOSB: "Women-Owned Small Business",
  EDWOSB: "Economically Disadvantaged WOSB",
  VSA: "Veteran-Owned Small Business",
  VSS: "Veteran-Owned SB Sole Source",
};

const RESPONSE_DEADLINE_OPTIONS = [
  { value: "", label: "Any deadline" },
  { value: "1m", label: "Next 1 month" },
  { value: "3m", label: "Next 3 months" },
  { value: "6m", label: "Next 6 months" },
  { value: "12m", label: "Next 12 months" },
];

export function SearchFilters({ stats }: { stats: StatsResponse }) {
  const searchParams = useSearchParams();
  // Key on searchParams so the inner component remounts (resetting all
  // local state) whenever the URL changes — no useEffect needed.
  return <SearchFiltersInner stats={stats} key={searchParams.toString()} />;
}

function SearchFiltersInner({ stats }: { stats: StatsResponse }) {
  const router = useRouter();
  const searchParams = useSearchParams();

  const [search, setSearch] = useState(searchParams.get("search") ?? "");
  const [naicsCodes, setNaicsCodes] = useState<string[]>(() => {
    const param = searchParams.get("naics_code") ?? "";
    return param ? param.split(",") : [];
  });
  const [oppTypes, setOppTypes] = useState<string[]>(() => {
    const param = searchParams.get("opp_type") ?? "";
    return param ? param.split(",") : [];
  });
  const [setAsides, setSetAsides] = useState<string[]>(() => {
    const param = searchParams.get("set_aside") ?? "";
    return param ? param.split(",") : [];
  });
  const [state, setState] = useState(searchParams.get("state") ?? "");
  const [department, setDepartment] = useState(searchParams.get("department") ?? "");
  const [responseDeadline, setResponseDeadline] = useState(searchParams.get("response_deadline") ?? "");
  const [activeOnly, setActiveOnly] = useState(searchParams.get("active_only") === "true");

  const toggleNaics = useCallback((code: string) => {
    setNaicsCodes((prev) =>
      prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]
    );
  }, []);

  const toggleOppType = useCallback((type: string) => {
    setOppTypes((prev) =>
      prev.includes(type) ? prev.filter((t) => t !== type) : [...prev, type]
    );
  }, []);

  const toggleSetAside = useCallback((code: string) => {
    setSetAsides((prev) =>
      prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]
    );
  }, []);

  const applyFilters = useCallback(() => {
    const params = new URLSearchParams();
    if (search) params.set("search", search);
    if (naicsCodes.length > 0) params.set("naics_code", naicsCodes.join(","));
    if (oppTypes.length > 0) params.set("opp_type", oppTypes.join(","));
    if (setAsides.length > 0) params.set("set_aside", setAsides.join(","));
    if (state) params.set("state", state);
    if (department) params.set("department", department);
    if (responseDeadline) params.set("response_deadline", responseDeadline);
    if (activeOnly) params.set("active_only", "true");
    const qs = params.toString();
    router.push(qs ? `/?${qs}` : "/");
  }, [search, naicsCodes, oppTypes, setAsides, state, department, responseDeadline, activeOnly, router]);

  const clearFilters = useCallback(() => {
    router.push("/");
  }, [router]);

  const topDepts = stats.departments.slice(0, 50);
  const topStates = stats.states.slice(0, 50);

  // Merge selected NAICS codes to the top, followed by the rest (up to 50 total)
  const naicsList = useMemo(() => {
    const selectedSet = new Set(naicsCodes);
    const selected = stats.naics_codes.filter((n) => selectedSet.has(n.value));
    const selectedValues = new Set(selected.map((n) => n.value));
    // Add selected codes that aren't in stats (e.g. from preset/URL) with count 0
    for (const code of naicsCodes) {
      if (!selectedValues.has(code)) {
        selected.push({ value: code, count: 0 });
      }
    }
    const rest = stats.naics_codes.filter((n) => !selectedSet.has(n.value));
    return [...selected, ...rest].slice(0, 50);
  }, [stats.naics_codes, naicsCodes]);

  return (
    <div className="space-y-4">
      <div>
        <Label htmlFor="search">Search</Label>
        <Input
          id="search"
          placeholder="Title, solicitation, dept..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && applyFilters()}
        />
      </div>

      <div>
        <Label>Response Deadline</Label>
        <Select
          value={responseDeadline || ALL_VALUE}
          onValueChange={(v) => setResponseDeadline(v === ALL_VALUE ? "" : v)}
        >
          <SelectTrigger>
            <SelectValue placeholder="Any deadline" />
          </SelectTrigger>
          <SelectContent>
            {RESPONSE_DEADLINE_OPTIONS.map((opt) => (
              <SelectItem key={opt.value || ALL_VALUE} value={opt.value || ALL_VALUE}>
                {opt.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div>
        <Label>Notice Type</Label>
        <div className="max-h-48 overflow-y-auto border border-border rounded-none p-2 space-y-1">
          {stats.opp_types.map((t) => (
            <div key={t.value} className="flex items-center gap-2">
              <Checkbox
                id={`opptype-${t.value}`}
                checked={oppTypes.includes(t.value)}
                onCheckedChange={() => toggleOppType(t.value)}
              />
              <Label htmlFor={`opptype-${t.value}`} className="text-sm font-normal cursor-pointer">
                {t.value} ({t.count})
              </Label>
            </div>
          ))}
        </div>
        {oppTypes.length > 0 && (
          <p className="text-xs text-muted-foreground mt-1">
            {oppTypes.length} selected
          </p>
        )}
      </div>

      <div>
        <Label>NAICS Code</Label>
        <div className="max-h-48 overflow-y-auto border border-border rounded-none p-2 space-y-1">
          {naicsList.map((n) => (
            <div key={n.value} className="flex items-center gap-2">
              <Checkbox
                id={`naics-${n.value}`}
                checked={naicsCodes.includes(n.value)}
                onCheckedChange={() => toggleNaics(n.value)}
              />
              <Label htmlFor={`naics-${n.value}`} className="text-sm font-normal cursor-pointer">
                {n.value}{NAICS_LABELS[n.value] ? ` — ${NAICS_LABELS[n.value]}` : ""} ({n.count})
              </Label>
            </div>
          ))}
        </div>
        {naicsCodes.length > 0 && (
          <p className="text-xs text-muted-foreground mt-1">
            {naicsCodes.length} selected
          </p>
        )}
      </div>

      {stats.set_asides.length > 0 && (
        <div>
          <Label>Set-Aside</Label>
          <div className="max-h-48 overflow-y-auto border border-border rounded-none p-2 space-y-1">
            {stats.set_asides.map((s) => (
              <div key={s.value} className="flex items-center gap-2">
                <Checkbox
                  id={`setaside-${s.value}`}
                  checked={setAsides.includes(s.value)}
                  onCheckedChange={() => toggleSetAside(s.value)}
                />
                <Label htmlFor={`setaside-${s.value}`} className="text-sm font-normal cursor-pointer">
                  {s.value}{SET_ASIDE_LABELS[s.value] ? ` — ${SET_ASIDE_LABELS[s.value]}` : ""} ({s.count})
                </Label>
              </div>
            ))}
          </div>
          {setAsides.length > 0 && (
            <p className="text-xs text-muted-foreground mt-1">
              {setAsides.length} selected
            </p>
          )}
        </div>
      )}

      <div>
        <Label>State</Label>
        <Select value={state || ALL_VALUE} onValueChange={(v) => setState(v === ALL_VALUE ? "" : v)}>
          <SelectTrigger>
            <SelectValue placeholder="All states" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VALUE}>All states</SelectItem>
            {topStates.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                {s.value} ({s.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div>
        <Label>Department</Label>
        <Select value={department || ALL_VALUE} onValueChange={(v) => setDepartment(v === ALL_VALUE ? "" : v)}>
          <SelectTrigger>
            <SelectValue placeholder="All departments" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VALUE}>All departments</SelectItem>
            {topDepts.map((d) => (
              <SelectItem key={d.value} value={d.value}>
                {d.value} ({d.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex items-center gap-2">
        <Checkbox
          id="active-only"
          checked={activeOnly}
          onCheckedChange={(checked) => setActiveOnly(checked === true)}
        />
        <Label htmlFor="active-only">Active only</Label>
      </div>

      <div className="flex gap-2">
        <Button onClick={applyFilters} className="flex-1">
          Apply
        </Button>
        <Button variant="outline" onClick={clearFilters}>
          Clear
        </Button>
      </div>
    </div>
  );
}
