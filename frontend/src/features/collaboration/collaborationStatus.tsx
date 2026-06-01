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
      return "bg-green-100 text-green-800";
    case "pending_buyer":
    case "pending_publisher":
      return "bg-amber-100 text-amber-800";
    case "revoked":
      return "bg-gray-100 text-gray-600";
    default:
      return "bg-gray-100 text-gray-600";
  }
}
