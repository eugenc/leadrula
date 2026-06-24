import { useRef, useState } from "react";
import { Phone, GitBranch } from "lucide-react";
import { Badge } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import type { AccountType, Lead } from "@/types";
import {
  cellValue,
  formatStatus,
  formatBuyerStatus,
  buyerStatusBadgeVariant,
} from "./leadsListColumns";

const ACTION_WIDTH = 140;
const SWIPE_THRESHOLD = 60;

const statusVariant: Record<
  string,
  "distributed" | "returned" | "review" | "closed" | "default"
> = {
  distributed: "distributed",
  returned: "returned",
  review: "review",
  closed: "closed",
};

interface LeadListCardProps {
  lead: Lead;
  accountType?: AccountType;
  isBuyer: boolean;
  onOpen: () => void;
  onChangeStage: () => void;
}

export function LeadListCard({
  lead,
  accountType,
  isBuyer,
  onOpen,
  onChangeStage,
}: LeadListCardProps) {
  const [offset, setOffset] = useState(0);
  const [open, setOpen] = useState(false);
  const startX = useRef(0);
  const startOffset = useRef(0);
  const swiping = useRef(false);

  const name = cellValue(lead, "name", []);
  const phone = lead.phone?.trim() ?? "";
  const stage = cellValue(lead, "stage", []);
  const created = cellValue(lead, "created_at", []);
  const assignee = lead.assignee_name;

  function snap(next: number, revealed: boolean) {
    setOffset(next);
    setOpen(revealed);
  }

  function onTouchStart(e: React.TouchEvent) {
    startX.current = e.touches[0].clientX;
    startOffset.current = offset;
    swiping.current = false;
  }

  function onTouchMove(e: React.TouchEvent) {
    const delta = e.touches[0].clientX - startX.current;
    if (Math.abs(delta) > 8) swiping.current = true;
    const next = Math.min(0, Math.max(-ACTION_WIDTH, startOffset.current + delta));
    setOffset(next);
  }

  function onTouchEnd() {
    if (!swiping.current) return;
    if (offset < -SWIPE_THRESHOLD) {
      snap(-ACTION_WIDTH, true);
    } else {
      snap(0, false);
    }
  }

  function handleTap() {
    if (open) {
      snap(0, false);
      return;
    }
    if (!swiping.current) onOpen();
  }

  return (
    <div className="relative overflow-hidden rounded-lg border border-gray-100 bg-surface-card">
      <div
        className="absolute inset-y-0 right-0 flex items-stretch"
        style={{ width: ACTION_WIDTH }}
      >
        {phone ? (
          <a
            href={`tel:${phone}`}
            className="flex flex-1 flex-col items-center justify-center gap-1 bg-jade-600 text-xs font-medium text-white"
            onClick={(e) => e.stopPropagation()}
          >
            <Phone className="h-4 w-4" />
            Call
          </a>
        ) : (
          <span className="flex flex-1 flex-col items-center justify-center gap-1 bg-gray-200 text-xs text-gray-400">
            <Phone className="h-4 w-4" />
            Call
          </span>
        )}
        <button
          type="button"
          className="flex flex-1 flex-col items-center justify-center gap-1 bg-gray-700 text-xs font-medium text-white"
          onClick={(e) => {
            e.stopPropagation();
            snap(0, false);
            onChangeStage();
          }}
        >
          <GitBranch className="h-4 w-4" />
          Stage
        </button>
      </div>

      <button
        type="button"
        className="relative w-full touch-pan-y pl-4 py-3 text-left transition-transform duration-150 ease-out"
        style={{ transform: `translateX(${offset}px)`, paddingRight: ACTION_WIDTH + 16 }}
        onTouchStart={onTouchStart}
        onTouchMove={onTouchMove}
        onTouchEnd={onTouchEnd}
        onClick={handleTap}
      >
        <div className="flex items-start justify-between gap-2">
          <span className="font-medium text-gray-800">{name}</span>
          <Badge
            plain
            variant={
              isBuyer ? buyerStatusBadgeVariant(lead) : statusVariant[lead.status] ?? "default"
            }
          >
            {isBuyer ? formatBuyerStatus(lead) : formatStatus(lead.status, accountType)}
          </Badge>
        </div>
        <div className="mt-1.5 space-y-0.5 text-sm text-gray-500">
          {stage !== "—" && <div>Stage: {stage}</div>}
          {phone && <div>{phone}</div>}
          <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-gray-400">
            <span>{created}</span>
            {assignee && <span>{assignee}</span>}
          </div>
        </div>
      </button>
    </div>
  );
}
