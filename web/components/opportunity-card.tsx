import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { OpportunityRow } from "@/lib/types";

export function OpportunityCard({ opp }: { opp: OpportunityRow }) {
  const noticeId = opp.notice_id ?? "";

  return (
    <Link href={`/opportunities/${encodeURIComponent(noticeId)}`}>
      <Card className="hover:translate-x-[2px] hover:translate-y-[2px] hover:shadow-none transition-all cursor-pointer">
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between gap-2">
            <CardTitle className="text-base leading-snug">
              {opp.title ?? "Untitled"}
            </CardTitle>
            {opp.active === "Yes" && (
              <Badge variant="default" className="shrink-0">Active</Badge>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            {opp.department ?? "—"}
            {opp.sub_tier ? ` / ${opp.sub_tier}` : ""}
          </p>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap gap-2 text-sm">
            {opp.base_type && (
              <Badge variant="secondary">{opp.base_type}</Badge>
            )}
            {opp.naics_code && (
              <Badge variant="outline">NAICS {opp.naics_code}</Badge>
            )}
            {opp.set_aside && (
              <Badge variant="outline">{opp.set_aside}</Badge>
            )}
            {opp.pop_state_code && (
              <Badge variant="outline">{opp.pop_state_code}</Badge>
            )}
          </div>
          <div className="mt-3 flex gap-4 text-xs text-muted-foreground">
            {opp.posted_date && <span>Posted: {opp.posted_date}</span>}
            {opp.response_deadline && (
              <span>Deadline: {opp.response_deadline}</span>
            )}
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
