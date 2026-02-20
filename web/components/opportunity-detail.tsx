import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Button } from "@/components/ui/button";
import { DownloadAllButton } from "@/components/download-all-button";
import type { DetailResponse } from "@/lib/types";

function Field({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <div className="grid grid-cols-[180px_1fr] gap-2 text-sm">
      <span className="font-semibold">{label}</span>
      <span>{value}</span>
    </div>
  );
}

function stripHtml(html: string): string {
  return html
    .replace(/<[^>]*>/g, "")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&nbsp;/g, " ")
    .replace(/&#39;/g, "'")
    .replace(/&quot;/g, '"')
    .replace(/\n{3,}/g, "\n\n");
}

export function OpportunityDetailView({ data }: { data: DetailResponse }) {
  const opp = data.opportunity;
  const contacts = data.contacts;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{opp.title ?? "Untitled"}</h1>
        <div className="mt-2 flex flex-wrap gap-2">
          {opp.active === "Yes" && <Badge variant="default">Active</Badge>}
          {opp.base_type && <Badge variant="secondary">{opp.base_type}</Badge>}
          {opp.opp_type && <Badge variant="outline">{opp.opp_type}</Badge>}
          {opp.set_aside && <Badge variant="outline">{opp.set_aside}</Badge>}
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Details</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Field label="Notice ID" value={opp.notice_id} />
          <Field label="Solicitation #" value={opp.solicitation_number} />
          <Field label="Type" value={opp.opp_type} />
          <Field label="Base Type" value={opp.base_type} />
          <Field label="Active" value={opp.active} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Organization</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Field label="Organization" value={opp.full_parent_path_name} />
          <Field label="Department" value={opp.department} />
          <Field label="Sub-tier" value={opp.sub_tier} />
          <Field label="Office" value={opp.office} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Dates</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Field label="Posted" value={opp.posted_date} />
          <Field label="Response Deadline" value={opp.response_deadline} />
          <Field label="Archive Date" value={opp.archive_date} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Classification</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Field label="NAICS Code" value={opp.naics_code} />
          <Field label="Classification Code" value={opp.classification_code} />
          <Field label="Set-Aside" value={opp.set_aside} />
          <Field label="Set-Aside Desc" value={opp.set_aside_description} />
        </CardContent>
      </Card>

      {(opp.pop_state_name || opp.pop_city_name || opp.pop_country_name || opp.pop_zip) && (
        <Card>
          <CardHeader>
            <CardTitle>Place of Performance</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <Field label="City" value={opp.pop_city_name} />
            <Field label="State" value={opp.pop_state_name} />
            <Field label="Country" value={opp.pop_country_name} />
            <Field label="ZIP" value={opp.pop_zip} />
          </CardContent>
        </Card>
      )}

      {contacts.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Contacts</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {contacts.map((c, i) => (
              <div key={i}>
                {i > 0 && <Separator className="mb-4" />}
                <div className="space-y-1 text-sm">
                  <p className="font-semibold">
                    {c.contact_type && <span className="text-muted-foreground">[{c.contact_type}] </span>}
                    {c.full_name ?? "Unknown"}
                  </p>
                  {c.title && <p className="text-muted-foreground">{c.title}</p>}
                  {c.email && <p>{c.email}</p>}
                  {c.phone && <p>{c.phone}</p>}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {(opp.award_amount || opp.awardee_name) && (
        <Card>
          <CardHeader>
            <CardTitle>Award</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            <Field label="Amount" value={opp.award_amount} />
            <Field label="Date" value={opp.award_date} />
            <Field label="Number" value={opp.award_number} />
            <Field label="Awardee" value={opp.awardee_name} />
            <Field label="UEI" value={opp.awardee_uei_sam} />
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Links</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {opp.ui_link && (
            <a href={opp.ui_link} target="_blank" rel="noopener noreferrer">
              <Button variant="default">View on SAM.gov</Button>
            </a>
          )}
          {opp.resource_links && opp.resource_links.length > 0 && (
            <div className="mt-2 space-y-1">
              <div className="flex items-center gap-2">
                <p className="text-sm font-semibold">Resources:</p>
                <DownloadAllButton opportunityId={opp.notice_id!} />
              </div>
              {opp.resource_links.map((link, i) => (
                <a
                  key={i}
                  href={link}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block text-sm text-primary underline break-all"
                >
                  {link}
                </a>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {opp.description && (
        <Card>
          <CardHeader>
            <CardTitle>Description</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm whitespace-pre-wrap">{stripHtml(opp.description)}</p>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
