"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
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

const ALL_VALUE = "__all__";

export function SearchFilters({ stats }: { stats: StatsResponse }) {
  const router = useRouter();
  const searchParams = useSearchParams();

  const [search, setSearch] = useState(searchParams.get("search") ?? "");
  const [naicsCodes, setNaicsCodes] = useState<string[]>(() => {
    const param = searchParams.get("naics_code") ?? "";
    return param ? param.split(",") : [];
  });
  const [oppType, setOppType] = useState(searchParams.get("opp_type") ?? "");
  const [setAside, setSetAside] = useState(searchParams.get("set_aside") ?? "");
  const [state, setState] = useState(searchParams.get("state") ?? "");
  const [department, setDepartment] = useState(searchParams.get("department") ?? "");
  const [activeOnly, setActiveOnly] = useState(searchParams.get("active_only") === "true");

  // Sync filter state from URL when searchParams change (e.g. preset navigation)
  useEffect(() => {
    setSearch(searchParams.get("search") ?? "");
    const naicsParam = searchParams.get("naics_code") ?? "";
    setNaicsCodes(naicsParam ? naicsParam.split(",") : []);
    setOppType(searchParams.get("opp_type") ?? "");
    setSetAside(searchParams.get("set_aside") ?? "");
    setState(searchParams.get("state") ?? "");
    setDepartment(searchParams.get("department") ?? "");
    setActiveOnly(searchParams.get("active_only") === "true");
  }, [searchParams]);

  const toggleNaics = useCallback((code: string) => {
    setNaicsCodes((prev) =>
      prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code]
    );
  }, []);

  const applyFilters = useCallback(() => {
    const params = new URLSearchParams();
    if (search) params.set("search", search);
    if (naicsCodes.length > 0) params.set("naics_code", naicsCodes.join(","));
    if (oppType) params.set("opp_type", oppType);
    if (setAside) params.set("set_aside", setAside);
    if (state) params.set("state", state);
    if (department) params.set("department", department);
    if (activeOnly) params.set("active_only", "true");
    const qs = params.toString();
    router.push(qs ? `/?${qs}` : "/");
  }, [search, naicsCodes, oppType, setAside, state, department, activeOnly, router]);

  const clearFilters = useCallback(() => {
    setSearch("");
    setNaicsCodes([]);
    setOppType("");
    setSetAside("");
    setState("");
    setDepartment("");
    setActiveOnly(false);
    router.push("/");
  }, [router]);

  const topNaics = stats.naics_codes.slice(0, 50);
  const topDepts = stats.departments.slice(0, 50);
  const topStates = stats.states.slice(0, 50);

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
        <Label>Type</Label>
        <Select value={oppType || ALL_VALUE} onValueChange={(v) => setOppType(v === ALL_VALUE ? "" : v)}>
          <SelectTrigger>
            <SelectValue placeholder="All types" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VALUE}>All types</SelectItem>
            {stats.opp_types.map((t) => (
              <SelectItem key={t.value} value={t.value}>
                {t.value} ({t.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div>
        <Label>NAICS Code</Label>
        <div className="max-h-48 overflow-y-auto border border-border rounded-none p-2 space-y-1">
          {topNaics.map((n) => (
            <div key={n.value} className="flex items-center gap-2">
              <Checkbox
                id={`naics-${n.value}`}
                checked={naicsCodes.includes(n.value)}
                onCheckedChange={() => toggleNaics(n.value)}
              />
              <Label htmlFor={`naics-${n.value}`} className="text-sm font-normal cursor-pointer">
                {n.value} ({n.count})
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

      <div>
        <Label>Set-Aside</Label>
        <Select value={setAside || ALL_VALUE} onValueChange={(v) => setSetAside(v === ALL_VALUE ? "" : v)}>
          <SelectTrigger>
            <SelectValue placeholder="All set-asides" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_VALUE}>All set-asides</SelectItem>
            {stats.set_asides.map((s) => (
              <SelectItem key={s.value} value={s.value}>
                {s.value} ({s.count})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

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
