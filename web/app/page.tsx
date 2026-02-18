import { Suspense } from "react";
import { fetchOpportunities, fetchStats } from "@/lib/api";
import { OpportunityCard } from "@/components/opportunity-card";
import { SearchFilters } from "@/components/search-filters";
import { FilterPresetBar } from "@/components/filter-preset-bar";
import { Pagination } from "@/components/pagination";
import { DEFAULT_PRESET } from "@/lib/presets";
import { Navbar } from "@/components/navbar";

interface PageProps {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}

export default async function Home({ searchParams }: PageProps) {
  const params = await searchParams;

  // If no URL params at all, apply default preset filters
  const hasParams = Object.keys(params).length > 0;
  const effectiveParams = hasParams ? params : DEFAULT_PRESET.filters;

  const filters = {
    search: typeof effectiveParams.search === "string" ? effectiveParams.search : undefined,
    naics_code: typeof effectiveParams.naics_code === "string" ? effectiveParams.naics_code : undefined,
    opp_type: typeof effectiveParams.opp_type === "string" ? effectiveParams.opp_type : undefined,
    set_aside: typeof effectiveParams.set_aside === "string" ? effectiveParams.set_aside : undefined,
    state: typeof effectiveParams.state === "string" ? effectiveParams.state : undefined,
    department: typeof effectiveParams.department === "string" ? effectiveParams.department : undefined,
    active_only: effectiveParams.active_only === "true",
    limit: 25,
    offset: typeof effectiveParams.offset === "string" ? parseInt(effectiveParams.offset, 10) || 0 : 0,
  };

  const [data, stats] = await Promise.all([
    fetchOpportunities(filters),
    fetchStats(),
  ]);

  return (
    <div className="min-h-screen">
      <Navbar currentPage="opportunities" />

      <div className="flex gap-6 p-6">
        <aside className="w-64 shrink-0">
          <Suspense fallback={<div>Loading filters...</div>}>
            <SearchFilters stats={stats} />
          </Suspense>
        </aside>

        <main className="flex-1 space-y-4">
          <Suspense fallback={null}>
            <FilterPresetBar />
          </Suspense>

          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {data.total.toLocaleString()} results
            </p>
          </div>

          <div className="grid gap-4">
            {data.opportunities.map((opp) => (
              <OpportunityCard key={opp.notice_id} opp={opp} />
            ))}
          </div>

          {data.opportunities.length === 0 && (
            <p className="text-center text-muted-foreground py-12">
              No opportunities match your filters.
            </p>
          )}

          <Suspense fallback={null}>
            <Pagination
              total={data.total}
              limit={data.limit}
              offset={data.offset}
            />
          </Suspense>
        </main>
      </div>
    </div>
  );
}
