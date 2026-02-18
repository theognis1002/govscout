"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { PRESETS } from "@/lib/presets";

export function FilterPresetBar() {
  const router = useRouter();
  const searchParams = useSearchParams();

  function isActive(presetFilters: Record<string, string>): boolean {
    const keys = new Set([
      ...Object.keys(presetFilters),
      ...Array.from(searchParams.keys()),
    ]);
    // Ignore offset when comparing
    keys.delete("offset");
    for (const key of keys) {
      const presetVal = presetFilters[key] ?? "";
      const urlVal = searchParams.get(key) ?? "";
      if (presetVal !== urlVal) return false;
    }
    return true;
  }

  function applyPreset(filters: Record<string, string>) {
    const params = new URLSearchParams(filters);
    router.push(`/?${params.toString()}`);
  }

  const hasAnyParams = searchParams.toString() !== "";

  return (
    <div className="flex flex-wrap gap-2">
      {PRESETS.map((preset) => (
        <Button
          key={preset.id}
          variant={isActive(preset.filters) ? "default" : "outline"}
          size="sm"
          onClick={() => applyPreset(preset.filters)}
        >
          {preset.label}
        </Button>
      ))}
      {hasAnyParams && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => router.push("/")}
        >
          Show All
        </Button>
      )}
    </div>
  );
}
