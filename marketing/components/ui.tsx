import Link from "next/link";
import type { ReactNode } from "react";

export const containerClass = "mx-auto max-w-[1020px] px-4 sm:px-6 lg:px-10";

export function Eyebrow({ children }: { children: ReactNode }) {
  return (
    <div className="mb-2.5 text-[11px] font-bold uppercase tracking-[1.2px] text-jade-600">
      {children}
    </div>
  );
}

export function H2({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <h2
      className={`mb-2.5 text-2xl font-extrabold leading-tight tracking-tight text-gray-800 sm:text-3xl ${className}`}
    >
      {children}
    </h2>
  );
}

export function Sub({ children, center = false }: { children: ReactNode; center?: boolean }) {
  return (
    <p className={`max-w-[480px] text-sm leading-relaxed text-gray-400 ${center ? "mx-auto" : ""}`}>
      {children}
    </p>
  );
}

export function Check() {
  return (
    <div className="mt-0.5 flex h-[15px] w-[15px] shrink-0 items-center justify-center rounded-full border border-jade-200 bg-jade-50">
      <svg viewBox="0 0 10 10" className="h-2 w-2 stroke-jade-500" fill="none" strokeWidth={2.5}>
        <polyline points="1.5,5 4,7.5 8.5,2.5" />
      </svg>
    </div>
  );
}

export function CheckItem({ label, text }: { label?: string; text: string }) {
  return (
    <li className="flex items-start gap-2 text-xs font-medium text-gray-600">
      <Check />
      <span>
        {label && <b className="font-semibold text-gray-700">{label}</b>}
        {label ? ` - ${text}` : text}
      </span>
    </li>
  );
}

export function QuoteButton({
  xl = false,
  ghost = false,
  white = false,
  children = "Get a Quote",
}: {
  xl?: boolean;
  ghost?: boolean;
  white?: boolean;
  children?: ReactNode;
}) {
  const base = "inline-flex items-center justify-center rounded-md font-medium transition-colors";
  const size = xl ? "h-[42px] rounded-lg px-6 text-sm font-semibold" : "h-8 px-3.5 text-[13px]";
  const color = white
    ? "bg-white text-jade-700 hover:bg-jade-50"
    : ghost
      ? "border border-gray-100 text-gray-600 hover:bg-gray-50"
      : "bg-jade-500 text-white hover:bg-jade-600";
  return (
    <Link href="/contact" className={`${base} ${size} ${color}`}>
      {children}
    </Link>
  );
}

export function Stat({ n, l }: { n: string; l: string }) {
  return (
    <div className="border-b border-gray-100 px-4 py-6 text-center last:border-b-0 md:border-b-0 md:border-r md:last:border-r-0">
      <span className="mb-1 block text-[26px] font-extrabold tracking-tight text-jade-500">{n}</span>
      <span className="text-[11px] font-medium text-gray-400">{l}</span>
    </div>
  );
}

export function StatBar({ stats }: { stats: [string, string][] }) {
  return (
    <div className="mt-12 grid grid-cols-2 overflow-hidden rounded-xl border border-gray-100 bg-white md:grid-cols-4">
      {stats.map(([n, l]) => (
        <Stat key={l} n={n} l={l} />
      ))}
    </div>
  );
}

export function FeatCard({ title, desc, icon }: { title: string; desc: string; icon?: ReactNode }) {
  return (
    <div className="rounded-lg border border-gray-100 bg-white p-[18px] transition-colors hover:border-jade-300">
      {icon && (
        <div className="mb-3 flex h-8 w-8 items-center justify-center rounded-lg border border-jade-100 bg-jade-50 text-jade-500">
          {icon}
        </div>
      )}
      <div className="mb-1 text-xs font-bold text-gray-800">{title}</div>
      <div className="text-[11px] leading-relaxed text-gray-400">{desc}</div>
    </div>
  );
}

export function MockCard({
  title,
  badge,
  badgeVariant = "jade",
  sub,
  bar,
}: {
  title: string;
  badge: string;
  badgeVariant?: "jade" | "info" | "warn" | "neutral";
  sub: string;
  bar?: number;
}) {
  const badgeCls = {
    jade: "border-jade-200 bg-jade-50 text-jade-700",
    info: "border-info-border bg-info-bg text-info",
    warn: "border-warning-border bg-warning-bg text-warning",
    neutral: "border-gray-200 bg-gray-50 text-gray-600",
  }[badgeVariant];
  return (
    <div className="mb-2.5 rounded-lg border border-gray-100 bg-white p-3.5 last:mb-0">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-800">{title}</span>
        <span
          className={`whitespace-nowrap rounded-full border px-2 py-0.5 text-[10px] font-semibold ${badgeCls}`}
        >
          {badge}
        </span>
      </div>
      <div className="mt-0.5 text-[10px] text-gray-300">{sub}</div>
      {bar !== undefined && (
        <div className="mt-2.5 h-[5px] overflow-hidden rounded bg-gray-100">
          <i className="block h-full rounded bg-jade-400" style={{ width: `${bar}%` }} />
        </div>
      )}
    </div>
  );
}

export function Mock({ children }: { children: ReactNode }) {
  return <div className="rounded-xl border border-gray-100 bg-gray-50 p-5">{children}</div>;
}

export function Dive({
  eyebrow,
  title,
  desc,
  items,
  mock,
  flip = false,
  id,
}: {
  eyebrow?: string;
  title: string;
  desc: string;
  items: { label?: string; text: string }[];
  mock: ReactNode;
  flip?: boolean;
  id?: string;
}) {
  const text = (
    <div>
      {eyebrow && <Eyebrow>{eyebrow}</Eyebrow>}
      <div className="mb-2.5 text-lg font-bold leading-snug tracking-tight text-gray-800 sm:text-[21px]">
        {title}
      </div>
      <div className="mb-4 text-[13px] leading-relaxed text-gray-400">{desc}</div>
      <ul className="flex flex-col gap-2">
        {items.map((it) => (
          <CheckItem key={it.text} label={it.label} text={it.text} />
        ))}
      </ul>
    </div>
  );
  return (
    <div
      id={id}
      className="grid grid-cols-1 items-center gap-8 border-b border-gray-100 py-10 last:border-b-0 lg:grid-cols-2 lg:gap-14 lg:py-13"
    >
      {flip ? (
        <>
          {mock}
          {text}
        </>
      ) : (
        <>
          {text}
          {mock}
        </>
      )}
    </div>
  );
}

export function XCard({
  vertical,
  name,
  meta,
  rows,
}: {
  vertical: string;
  name: string;
  meta: string;
  rows: [string, string][];
}) {
  return (
    <div className="rounded-lg border border-gray-100 bg-white p-[18px] transition-all hover:border-jade-300 hover:shadow-lg hover:shadow-jade-500/5">
      <div className="mb-3 flex items-center justify-between">
        <span className="rounded-full border border-jade-200 bg-jade-50 px-2 py-0.5 text-[10px] font-semibold text-jade-700">
          {vertical}
        </span>
        <div className="h-1.5 w-1.5 rounded-full bg-jade-500" />
      </div>
      <div className="mb-0.5 text-[13px] font-bold text-gray-800">{name}</div>
      <div className="mb-3 text-[11px] text-gray-300">{meta}</div>
      {rows.map(([k, v]) => (
        <div key={k} className="flex justify-between border-t border-gray-100 py-1.5 text-[11px]">
          <span className="text-gray-400">{k}</span>
          <span className="font-semibold text-gray-700">{v}</span>
        </div>
      ))}
    </div>
  );
}

export function Pill({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-center gap-1.5 rounded-lg border border-gray-100 bg-white px-3.5 py-[7px] text-xs font-semibold text-gray-600">
      <div className="h-[5px] w-[5px] rounded-full bg-jade-400" />
      {children}
    </div>
  );
}

export function LeadTypeCard({
  icon,
  title,
  tag,
  desc,
  items,
}: {
  icon: ReactNode;
  title: string;
  tag: string;
  desc: string;
  items: string[];
}) {
  return (
    <div className="rounded-xl border border-gray-100 bg-white p-6 transition-all hover:border-jade-300 hover:shadow-lg hover:shadow-jade-500/10">
      <div className="mb-4 flex items-center justify-between">
        <div className="flex h-[38px] w-[38px] items-center justify-center rounded-[9px] border border-jade-100 bg-jade-50 text-jade-500">
          {icon}
        </div>
        <span className="rounded-full border border-jade-200 bg-jade-50 px-2 py-0.5 text-[10px] font-semibold text-jade-700">
          {tag}
        </span>
      </div>
      <div className="mb-1.5 text-[15px] font-bold text-gray-800">{title}</div>
      <div className="mb-4 text-xs leading-relaxed text-gray-400">{desc}</div>
      <ul className="flex flex-col gap-2">
        {items.map((t) => (
          <CheckItem key={t} text={t} />
        ))}
      </ul>
    </div>
  );
}

export const LEAD_TYPES = [
  {
    title: "Data",
    tag: "Web leads",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round">
        <ellipse cx="12" cy="5" rx="9" ry="3" />
        <path d="M3 5v14a9 3 0 0 0 18 0V5" />
        <path d="M3 12a9 3 0 0 0 18 0" />
      </svg>
    ),
    desc: "Web leads from any source, validated and enriched before they reach a buyer.",
    items: ["API, web forms, or CSV import", "Custom fields per vertical", "Dedupe and validation on entry"],
  },
  {
    title: "Appointments",
    tag: "Booked",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="18" rx="2" />
        <line x1="16" y1="2" x2="16" y2="6" />
        <line x1="8" y1="2" x2="8" y2="6" />
        <line x1="3" y1="10" x2="21" y2="10" />
        <path d="M9 16l2 2 4-4" />
      </svg>
    ),
    desc: "Booked appointments distributed with the time slot, synced to the buyer's calendar.",
    items: ["Booked from the pipeline", "Synced to buyer calendars", "Reminders and no-show handling"],
  },
  {
    title: "Calls",
    tag: "Pay-per-call",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z" />
      </svg>
    ),
    desc: "Inbound calls routed live to buyers by real-time bid or static rules, billed by duration.",
    items: ["RTB and static call routing", "Per-second / per-minute billing", "Connect threshold before charge"],
  },
];
