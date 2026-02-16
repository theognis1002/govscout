import { Suspense } from "react";
import { fetchOpportunities, fetchStats } from "@/lib/api";
import { OpportunityCard } from "@/components/opportunity-card";
import { SearchFilters } from "@/components/search-filters";
import { Pagination } from "@/components/pagination";

interface PageProps {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}

export default async function Home({ searchParams }: PageProps) {
  const params = await searchParams;
  const filters = {
    search: typeof params.search === "string" ? params.search : undefined,
    naics_code: typeof params.naics_code === "string" ? params.naics_code : undefined,
    opp_type: typeof params.opp_type === "string" ? params.opp_type : undefined,
    set_aside: typeof params.set_aside === "string" ? params.set_aside : undefined,
    state: typeof params.state === "string" ? params.state : undefined,
    department: typeof params.department === "string" ? params.department : undefined,
    active_only: params.active_only === "true",
    limit: 25,
    offset: typeof params.offset === "string" ? parseInt(params.offset, 10) || 0 : 0,
  };

  const [data, stats] = await Promise.all([
    fetchOpportunities(filters),
    fetchStats(),
  ]);

  return (
    <div className="min-h-screen">
      <header className="border-b-2 border-border bg-background px-6 py-4">
        <h1 className="text-2xl font-bold">GovScout</h1>
        <p className="text-sm text-muted-foreground">
          {stats.total_opportunities.toLocaleString()} federal contract opportunities
        </p>
      </header>

      <div className="flex gap-6 p-6">
        <aside className="w-64 shrink-0">
          <Suspense fallback={<div>Loading filters...</div>}>
            <SearchFilters stats={stats} />
          </Suspense>
        </aside>

        <main className="flex-1 space-y-4">
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
