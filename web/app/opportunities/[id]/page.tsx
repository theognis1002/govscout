import Link from "next/link";
import { fetchOpportunity } from "@/lib/api";
import { OpportunityDetailView } from "@/components/opportunity-detail";
import { Button } from "@/components/ui/button";
import { Navbar } from "@/components/navbar";

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function OpportunityPage({ params }: PageProps) {
  const { id } = await params;
  const data = await fetchOpportunity(id);

  return (
    <div className="min-h-screen">
      <Navbar currentPage="detail" />

      <main className="mx-auto max-w-4xl p-6">
        <div className="mb-4">
          <Link href="/">
            <Button variant="outline" size="sm">
              &larr; Back to Opportunities
            </Button>
          </Link>
        </div>
        <OpportunityDetailView data={data} />
      </main>
    </div>
  );
}
