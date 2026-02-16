"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";

interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
}

export function Pagination({ total, limit, offset }: PaginationProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentPage = Math.floor(offset / limit) + 1;
  const totalPages = Math.ceil(total / limit);

  if (totalPages <= 1) return null;

  const goToPage = (page: number) => {
    const params = new URLSearchParams(searchParams.toString());
    const newOffset = (page - 1) * limit;
    if (newOffset > 0) {
      params.set("offset", newOffset.toString());
    } else {
      params.delete("offset");
    }
    const qs = params.toString();
    router.push(qs ? `/?${qs}` : "/");
  };

  const pageNumbers: number[] = [];
  const start = Math.max(1, currentPage - 2);
  const end = Math.min(totalPages, currentPage + 2);
  for (let i = start; i <= end; i++) {
    pageNumbers.push(i);
  }

  return (
    <div className="flex items-center justify-between gap-4">
      <p className="text-sm text-muted-foreground">
        {offset + 1}–{Math.min(offset + limit, total)} of {total}
      </p>
      <div className="flex gap-1">
        <Button
          variant="outline"
          size="sm"
          disabled={currentPage === 1}
          onClick={() => goToPage(currentPage - 1)}
        >
          Prev
        </Button>
        {start > 1 && (
          <>
            <Button variant="outline" size="sm" onClick={() => goToPage(1)}>
              1
            </Button>
            {start > 2 && <span className="px-2 text-muted-foreground">...</span>}
          </>
        )}
        {pageNumbers.map((p) => (
          <Button
            key={p}
            variant={p === currentPage ? "default" : "outline"}
            size="sm"
            onClick={() => goToPage(p)}
          >
            {p}
          </Button>
        ))}
        {end < totalPages && (
          <>
            {end < totalPages - 1 && <span className="px-2 text-muted-foreground">...</span>}
            <Button variant="outline" size="sm" onClick={() => goToPage(totalPages)}>
              {totalPages}
            </Button>
          </>
        )}
        <Button
          variant="outline"
          size="sm"
          disabled={currentPage === totalPages}
          onClick={() => goToPage(currentPage + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
