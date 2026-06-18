import type { AccountType, NotificationChannelPrefs, NotificationPrefs } from "@/types";

export type NotificationEventDef = {
  id: string;
  label: string;
};

export const LEAD_NOTIFICATION_EVENTS: NotificationEventDef[] = [
  { id: "new_lead", label: "New lead received" },
  { id: "lead_returned", label: "A lead was returned" },
];

export const BUYER_ACCOUNT_NOTIFICATION_EVENTS: NotificationEventDef[] = [
  { id: "dispute_update", label: "Dispute updates" },
  { id: "new_invoice", label: "New invoices" },
  { id: "collaboration_request", label: "Collaboration requests" },
  { id: "partnership_request", label: "Partnership requests" },
  { id: "partnership_accepted", label: "Partnership accepted" },
  { id: "contract_participation_pending", label: "Contract invitations" },
  { id: "contract_forked", label: "Counter-offer contracts" },
];

export const PUBLISHER_ACCOUNT_NOTIFICATION_EVENTS: NotificationEventDef[] = [
  { id: "collaboration_request", label: "Collaboration requests" },
  { id: "partnership_request", label: "Partnership requests" },
  { id: "partnership_accepted", label: "Partnership accepted" },
  { id: "contract_participation_accepted", label: "Contract accepted" },
  { id: "contract_participation_declined", label: "Contract declined" },
  { id: "contract_counter_pending", label: "Contract counter-offers" },
];

export function accountNotificationEvents(accountType: AccountType): NotificationEventDef[] {
  if (accountType === "buyer") return BUYER_ACCOUNT_NOTIFICATION_EVENTS;
  if (accountType === "publisher") return PUBLISHER_ACCOUNT_NOTIFICATION_EVENTS;
  return [];
}

export function defaultChannelPrefs(): NotificationChannelPrefs {
  return { in_app: true, email: false };
}

export function withDefaults(
  stored: NotificationPrefs | undefined,
  events: NotificationEventDef[],
): NotificationPrefs {
  const out: NotificationPrefs = {};
  for (const e of events) {
    out[e.id] = stored?.[e.id] ?? defaultChannelPrefs();
  }
  return out;
}
