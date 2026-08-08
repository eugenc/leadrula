import { memo, type ReactNode } from "react";
import type { AccountType, CustomField, Lead, StageType } from "@/types";
import { Avatar, Badge } from "@/components/ui/misc";
import { useAuthStore } from "@/store/authStore";
import { ActionIndicator } from "./ActionDot";
import { ReturnIndicator } from "./ReturnIndicator";
import { cellValue, columnIcon, buyerStatusBadgeVariant, formatBuyerStatus, formatStatus } from "./leadsListColumns";
import { showActionAtForStage } from "@/features/pipelines/stageTypes";
import { LeadTagBadges } from "./LeadTagsEditor";
import { cn } from "@/lib/utils";

const statusVariant: Record<
  string,
  "distributed" | "returned" | "review" | "closed" | "default"
> = {
  distributed: "distributed",
  returned: "returned",
  review: "review",
  closed: "closed",
};

function CardFieldLine({
  colId,
  customFields,
  children,
}: {
  colId: string;
  customFields: CustomField[];
  children: ReactNode;
}) {
  const Icon = columnIcon(colId, customFields);
  return (
    <div className="flex items-center gap-2 leading-tight">
      <span className="flex w-4 shrink-0 justify-center">
        <Icon className="h-3.5 w-3.5 text-gray-300" aria-hidden />
      </span>
      <div className="min-w-0 flex-1 text-xs text-gray-500">{children}</div>
    </div>
  );
}

function CardFieldRow({
  colId,
  lead,
  customFields,
  accountType,
}: {
  colId: string;
  lead: Lead;
  customFields: CustomField[];
  accountType?: AccountType;
}) {
  if (colId === "name") return null;

  if (colId === "tags") {
    if (!(lead.tags ?? []).length) return null;
    return (
      <CardFieldLine colId={colId} customFields={customFields}>
        <LeadTagBadges tags={lead.tags ?? []} limit={2} />
      </CardFieldLine>
    );
  }

  if (colId === "status") {
    const variant =
      accountType === "buyer"
        ? buyerStatusBadgeVariant(lead)
        : statusVariant[lead.status] ?? "default";
    const label =
      accountType === "buyer" ? formatBuyerStatus(lead) : formatStatus(lead.status, accountType);
    return (
      <CardFieldLine colId={colId} customFields={customFields}>
        <Badge variant={variant} plain>{label}</Badge>
      </CardFieldLine>
    );
  }

  const value = cellValue(lead, colId, customFields, accountType);
  if (!value || value === "—") return null;

  return (
    <CardFieldLine colId={colId} customFields={customFields}>
      <span className="truncate">{value}</span>
    </CardFieldLine>
  );
}

export const LeadCard = memo(function LeadCard({
  lead,
  customFields,
  cardFields,
  onClick,
  dragging,
  stageType,
}: {
  lead: Lead;
  customFields: CustomField[];
  cardFields: string[];
  onClick: () => void;
  dragging?: boolean;
  stageType?: StageType;
}) {
  const accountType = useAuthStore((s) => s.user?.account_type);
  const showActionAt = showActionAtForStage(stageType ?? lead.stage_type);
  const activeFields = cardFields.filter((id) => id !== "name");
  const showReturnedInHeader =
    lead.status === "returned" && !activeFields.includes("status");
  const showBuyerStage =
    (lead.status === "distributed" || lead.status === "closed") &&
    !!lead.stage_name &&
    !!lead.buyer_name;

  return (
    <div
      onClick={onClick}
      className={cn(
        "cursor-pointer rounded-lg border border-gray-100 bg-surface-card p-3 shadow-xs transition-[box-shadow,transform] hover:-translate-y-px hover:shadow-md",
        dragging && "rotate-[2deg] scale-[1.02] opacity-[0.92] shadow-xl"
      )}
    >
      <div className="mb-2 flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-sm font-semibold leading-snug text-gray-800">
          {lead.first_name} {lead.last_name}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          {showReturnedInHeader && (
            <Badge variant="returned" plain>
              {accountType === "buyer" ? formatBuyerStatus(lead) : formatStatus(lead.status, accountType)}
            </Badge>
          )}
          {showActionAt && <ActionIndicator actionAt={lead.action_at} size="sm" />}
          {lead.pending_return_at && (
            <ReturnIndicator
              pendingReturnAt={lead.pending_return_at}
              pendingReturnTimezone={lead.pending_return_timezone}
              variant="card"
            />
          )}
          {lead.assigned_user_id && (
            <span
              className="group/avatar relative shrink-0"
              onClick={(e) => e.stopPropagation()}
              onMouseDown={(e) => e.stopPropagation()}
            >
              <Avatar
                name={lead.assignee_name ?? ""}
                src={lead.assignee_avatar_url}
                variant="card"
              />
              {lead.assignee_name && (
                <span
                  role="tooltip"
                  className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1.5 -translate-x-1/2 whitespace-nowrap rounded-md bg-[#101828] px-2 py-1 text-xs font-medium text-[#F9FAFB] opacity-0 shadow-sm transition-opacity duration-150 group-hover/avatar:opacity-100"
                >
                  {lead.assignee_name}
                </span>
              )}
            </span>
          )}
        </div>
      </div>

      {showBuyerStage && (
        <p className="mb-2 text-xs text-gray-500">
          {lead.buyer_name}: {lead.stage_name}
        </p>
      )}

      {activeFields.length > 0 && (
        <div className="flex flex-col gap-1.5">
          {activeFields.map((colId) => (
            <CardFieldRow
              key={colId}
              colId={colId}
              lead={lead}
              customFields={customFields}
              accountType={accountType}
            />
          ))}
        </div>
      )}
    </div>
  );
});
