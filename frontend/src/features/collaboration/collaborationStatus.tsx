export function collabLabel(status: string) {
  switch (status) {
    case "active":
      return "Active";
    case "pending_buyer":
      return "Pending your approval";
    case "pending_publisher":
      return "Pending publisher approval";
    case "revoked":
      return "Revoked";
    default:
      return "None";
  }
}

export function collabBadgeClass(status: string) {
  switch (status) {
    case "active":
      return "border border-success-border bg-success-bg text-success-fg";
    case "pending_buyer":
    case "pending_publisher":
      return "border border-warning-border bg-warning-bg text-warning-fg";
    case "revoked":
      return "border border-neutral-border bg-neutral-bg text-neutral-fg";
    default:
      return "border border-neutral-border bg-neutral-bg text-neutral-fg";
  }
}
