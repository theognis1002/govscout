import { NextResponse } from "next/server";
import JSZip from "jszip";
import { fetchOpportunity } from "@/lib/api";

function toKebabCase(str: string): string {
  return str
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;

  const data = await fetchOpportunity(id);
  const opp = data.opportunity;
  const links = opp.resource_links ?? [];

  if (links.length === 0) {
    return NextResponse.json({ error: "No resource files" }, { status: 404 });
  }

  const solicitation = opp.solicitation_number ?? "unknown";
  const title = opp.title ?? "opportunity";
  const zipName = `${solicitation}-${toKebabCase(title)}.zip`;

  const zip = new JSZip();

  const results = await Promise.allSettled(
    links.map(async (url) => {
      const res = await fetch(url);
      if (!res.ok) throw new Error(`Failed to fetch ${url}: ${res.status}`);

      const contentDisposition = res.headers.get("content-disposition");
      let filename: string | null = null;
      if (contentDisposition) {
        const match = contentDisposition.match(/filename\*?=(?:UTF-8''|"?)([^";]+)"?/i);
        if (match) filename = decodeURIComponent(match[1]);
      }
      if (!filename) {
        // Use the file ID from the URL as fallback
        const segments = new URL(url).pathname.split("/");
        filename = segments[segments.length - 2] || segments[segments.length - 1];
      }

      const buffer = await res.arrayBuffer();
      return { filename, buffer };
    })
  );

  const seen = new Map<string, number>();
  for (const result of results) {
    if (result.status !== "fulfilled") continue;
    let { filename } = result.value;

    // Deduplicate filenames
    const count = seen.get(filename) ?? 0;
    seen.set(filename, count + 1);
    if (count > 0) {
      const dot = filename.lastIndexOf(".");
      if (dot > 0) {
        filename = `${filename.slice(0, dot)}-${count}${filename.slice(dot)}`;
      } else {
        filename = `${filename}-${count}`;
      }
    }

    zip.file(filename, result.value.buffer);
  }

  const zipBuffer = await zip.generateAsync({ type: "arraybuffer" });

  return new NextResponse(zipBuffer, {
    headers: {
      "Content-Type": "application/zip",
      "Content-Disposition": `attachment; filename="${zipName}"`,
    },
  });
}
