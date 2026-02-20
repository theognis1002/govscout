"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Download } from "lucide-react";

export function DownloadAllButton({ opportunityId }: { opportunityId: string }) {
  const [loading, setLoading] = useState(false);

  async function handleDownload() {
    setLoading(true);
    try {
      const res = await fetch(`/download/${encodeURIComponent(opportunityId)}`);
      if (!res.ok) throw new Error("Download failed");

      const disposition = res.headers.get("content-disposition");
      const match = disposition?.match(/filename="(.+)"/);
      const filename = match?.[1] ?? "resources.zip";

      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch {
      alert("Failed to download files. Please try again.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <Button variant="default" onClick={handleDownload} disabled={loading}>
      <Download className="mr-2 h-4 w-4" />
      {loading ? "Downloading..." : "Download All"}
    </Button>
  );
}
