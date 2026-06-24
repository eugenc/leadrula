// formatCallStatus turns a raw call status (e.g. "no_answer") into a display
// label with each word capitalized (e.g. "No Answer").
export function formatCallStatus(status: string): string {
  return status
    .split("_")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}
