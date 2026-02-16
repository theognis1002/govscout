import Link from "next/link";
import { fetchOpportunity } from "@/lib/api";
import { OpportunityDetailView } from "@/components/opportunity-detail";
import { Button } from "@/components/ui/button";

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function OpportunityPage({ params }: PageProps) {
  const { id } = await params;
  const data = await fetchOpportunity(id);

  return (
    <div className="min-h-screen">
      <header className="border-b-2 border-border bg-background px-6 py-4">
        <div className="flex items-center gap-4">
          <Link href="/">
            <Button variant="outline" size="sm">
              Back
            </Button>
          </Link>
          <h1 className="text-2xl font-bold">GovScout</h1>
        </div>
      </header>

      <main className="mx-auto max-w-4xl p-6">
        <OpportunityDetailView data={data} />
      </main>
    </div>
  );
}
