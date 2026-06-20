export const STAGE_COLORS = [
  { slug: "gray", dot: "bg-gray-400", ring: "ring-gray-400", border: "border-gray-400", line: "bg-gray-400/20", fill: "bg-gray-400/45" },
  { slug: "jade", dot: "bg-jade-500", ring: "ring-jade-500", border: "border-jade-500", line: "bg-jade-500/20", fill: "bg-jade-500/45" },
  { slug: "blue", dot: "bg-blue-500", ring: "ring-blue-500", border: "border-blue-500", line: "bg-blue-500/20", fill: "bg-blue-500/45" },
  { slug: "amber", dot: "bg-amber-500", ring: "ring-amber-500", border: "border-amber-500", line: "bg-amber-500/20", fill: "bg-amber-500/45" },
  { slug: "red", dot: "bg-red-500", ring: "ring-red-500", border: "border-red-500", line: "bg-red-500/20", fill: "bg-red-500/45" },
  { slug: "purple", dot: "bg-purple-500", ring: "ring-purple-500", border: "border-purple-500", line: "bg-purple-500/20", fill: "bg-purple-500/45" },
  { slug: "teal", dot: "bg-teal-500", ring: "ring-teal-500", border: "border-teal-500", line: "bg-teal-500/20", fill: "bg-teal-500/45" },
  { slug: "orange", dot: "bg-orange-500", ring: "ring-orange-500", border: "border-orange-500", line: "bg-orange-500/20", fill: "bg-orange-500/45" },
  { slug: "pink", dot: "bg-pink-500", ring: "ring-pink-500", border: "border-pink-500", line: "bg-pink-500/20", fill: "bg-pink-500/45" },
  { slug: "slate", dot: "bg-slate-500", ring: "ring-slate-500", border: "border-slate-500", line: "bg-slate-500/20", fill: "bg-slate-500/45" },
] as const;

export type StageColorSlug = (typeof STAGE_COLORS)[number]["slug"];

export function stageColorDot(slug: string | undefined): string {
  return STAGE_COLORS.find((c) => c.slug === slug)?.dot ?? "bg-gray-400";
}

export function stageColorBorder(slug: string | undefined): string {
  return STAGE_COLORS.find((c) => c.slug === slug)?.border ?? "border-gray-400";
}

export function stageColorLine(slug: string | undefined): string {
  return STAGE_COLORS.find((c) => c.slug === slug)?.line ?? "bg-gray-400/20";
}

export function stageColorFill(slug: string | undefined): string {
  return STAGE_COLORS.find((c) => c.slug === slug)?.fill ?? "bg-gray-400/45";
}
