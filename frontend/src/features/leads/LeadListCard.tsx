import { type ReactNode } from "react";
import { Phone, GitBranch, StickyNote } from "lucide-react";
import { Badge } from "@/components/ui/misc";
import { cn } from "@/lib/utils";
import type { AccountType, Lead } from "@/types";
import {
  cellValue,
  columnIcon,
  formatStatus,
  formatBuyerStatus,
  buyerStatusBadgeVariant,
} from "./leadsListColumns";

const ACTION_BTN =
  "flex w-[70px] shrink-0 snap-start flex-col items-center justify-center gap-1 text-xs font-medium";

const statusVariant: Record<
  string,
  "distributed" | "returned" | "review" | "closed" | "default"
> = {
  distributed: "distributed",
  returned: "returned",
  review: "review",
  closed: "closed",
};

function FieldLine({
  colId,
  children,
  className,
}: {
  colId: string;
  children: ReactNode;
  className?: string;
}) {
  const Icon = columnIcon(colId);
  return (
    <div className={cn("flex items-center gap-2 leading-tight", className)}>
      <span className="flex w-4 shrink-0 justify-center">
        <Icon className="h-3.5 w-3.5 text-gray-300" aria-hidden />
      </span>
      <span className="min-w-0 truncate">{children}</span>
    </div>
  );
}

interface LeadListCardProps {
  lead: Lead;
  accountType?: AccountType;
  isBuyer: boolean;
  onOpen: () => void;
  onChangeStage: () => void;
  onAddNote: () => void;
}

export function LeadListCard({
  lead,
  accountType,
  isBuyer,
  onOpen,
  onChangeStage,
  onAddNote,
}: LeadListCardProps) {
  const name = cellValue(lead, "name", []);
  const phone = lead.phone?.trim() ?? "";
  const pipeline = cellValue(lead, "pipeline", []);
  const stage = cellValue(lead, "stage", []);
  const created = cellValue(lead, "created_at", []);
  const assignee = lead.assignee_name;

  return (
    <div className="flex overflow-hidden rounded-lg border border-gray-100 bg-surface-card">
      <button type="button" className="min-w-0 flex-1 px-4 py-3 text-left" onClick={onOpen}>
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
        <div className="mt-1.5 space-y-0.5">
          {(pipeline !== "—" || stage !== "—") && (
            <FieldLine colId="stage" className="text-sm text-gray-500">
              {pipeline !== "—" && stage !== "—"
                ? `${pipeline} · ${stage}`
                : pipeline !== "—"
                  ? pipeline
                  : stage}
            </FieldLine>
          )}
          {phone && (
            <FieldLine colId="phone" className="text-sm text-gray-500">
              {phone}
            </FieldLine>
          )}
          <FieldLine colId="created_at" className="text-xs text-gray-400">
            {created}
          </FieldLine>
          {assignee && (
            <FieldLine colId="assignee" className="text-xs text-gray-400">
              {assignee}
            </FieldLine>
          )}
        </div>
      </button>

      <div
        className="flex w-[140px] shrink-0 snap-x snap-mandatory overflow-x-auto overscroll-x-contain [-webkit-overflow-scrolling:touch]"
        onTouchStart={(e) => e.stopPropagation()}
        onTouchMove={(e) => e.stopPropagation()}
      >
        {phone ? (
          <a
            href={`tel:${phone}`}
            className={cn(ACTION_BTN, "bg-jade-500 text-white")}
          >
            <Phone className="h-4 w-4" />
            Call
          </a>
        ) : (
          <span className={cn(ACTION_BTN, "cursor-default bg-gray-100 text-gray-400")}>
            <Phone className="h-4 w-4" />
            Call
          </span>
        )}
        <button
          type="button"
          className={cn(ACTION_BTN, "bg-surface-hover text-gray-800")}
          onClick={onAddNote}
        >
          <StickyNote className="h-4 w-4" />
          Note
        </button>
        <button
          type="button"
          className={cn(ACTION_BTN, "bg-gray-100 text-gray-800")}
          onClick={onChangeStage}
        >
          <GitBranch className="h-4 w-4" />
          Stage
        </button>
      </div>
    </div>
  );
}
