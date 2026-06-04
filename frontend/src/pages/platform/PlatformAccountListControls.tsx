import { Input, FilterSelect } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { PAGE_SIZES } from "@/features/intake/logShared";

interface SearchBarProps {
  search: string;
  onSearchChange: (value: string) => void;
  placeholder?: string;
}

export function PlatformAccountSearchBar({ search, onSearchChange, placeholder = "Search name or handler ID…" }: SearchBarProps) {
  return (
    <div className="mb-4">
      <Input
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder={placeholder}
        className="max-w-sm"
      />
    </div>
  );
}

interface PaginationProps {
  page: number;
  limit: number;
  total: number;
  onPageChange: (page: number) => void;
  onLimitChange: (limit: number) => void;
}

export function PlatformAccountPagination({ page, limit, total, onPageChange, onLimitChange }: PaginationProps) {
  if (total <= 0) return null;

  const totalPages = Math.max(1, Math.ceil(total / limit));

  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-gray-500">
      <span>
        {(page - 1) * limit + 1}–{Math.min(page * limit, total)} of {total}
      </span>
      <div className="flex items-center gap-3">
        <FilterSelect value={limit} onChange={(e) => onLimitChange(Number(e.target.value))} className="w-24">
          {PAGE_SIZES.map((n) => (
            <option key={n} value={n}>
              {n} / page
            </option>
          ))}
        </FilterSelect>
        <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>
          Previous
        </Button>
        <span>
          Page {page} of {totalPages}
        </span>
        <Button
          variant="secondary"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onPageChange(page + 1)}
        >
          Next
        </Button>
      </div>
    </div>
  );
}
