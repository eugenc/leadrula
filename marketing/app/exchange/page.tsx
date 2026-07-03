import type { Metadata } from "next";
import { pageMeta } from "@/lib/metadata";
import { containerClass, Eyebrow, H2, Pill, QuoteButton, XCard } from "@/components/ui";

export const metadata: Metadata = pageMeta({
  title: "Exchange — Lead Marketplace for Publishers & Buyers",
  description:
    "Browse vetted publishers and buyers across 30+ verticals. Send a partnership request, agree on a contract, start trading.",
  path: "/exchange",
});

const listings: {
  vertical: string;
  name: string;
  meta: string;
  rows: [string, string][];
}[] = [
  {
    vertical: "Solar",
    name: "SunPath Media",
    meta: "Ontario · Exclusive · 200/mo",
    rows: [
      ["Payout", "Rev share 20%"],
      ["Delivery", "Real-time API"],
      ["Acceptance", "94%"],
    ],
  },
  {
    vertical: "Insurance",
    name: "PolicyBridge",
    meta: "North America · Multi-sell · 500/mo",
    rows: [
      ["Payout", "$28 flat/lead"],
      ["Delivery", "Ping-post"],
      ["Acceptance", "88%"],
    ],
  },
  {
    vertical: "Mortgage",
    name: "LendSource Partners",
    meta: "US · Exclusive · 150/mo",
    rows: [
      ["Payout", "Rev share 15%"],
      ["Delivery", "CRM direct"],
      ["Acceptance", "91%"],
    ],
  },
  {
    vertical: "HVAC",
    name: "ComfortCall Group",
    meta: "Texas · Exclusive · 300/mo",
    rows: [
      ["Payout", "$42 flat/lead"],
      ["Delivery", "Real-time API"],
      ["Acceptance", "92%"],
    ],
  },
  {
    vertical: "Roofing",
    name: "PeakLine Buyers",
    meta: "Florida · Multi-sell · 250/mo",
    rows: [
      ["Payout", "Rev share 12%"],
      ["Delivery", "Webhook"],
      ["Acceptance", "89%"],
    ],
  },
  {
    vertical: "Legal",
    name: "ClaimBridge Law",
    meta: "US · Exclusive · 80/mo",
    rows: [
      ["Payout", "$120 flat/lead"],
      ["Delivery", "CRM direct"],
      ["Acceptance", "96%"],
    ],
  },
];

const verticals = [
  "Solar",
  "Insurance",
  "Mortgage",
  "HVAC",
  "Roofing",
  "Home Services",
  "Legal",
  "Real Estate",
  "Healthcare",
  "Finance",
  "Auto",
  "Education",
];

const steps = [
  {
    title: "1. Discover",
    desc: "Browse listings by vertical, geography, cap, and payout type. Every listing shows real acceptance rates.",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
        <circle cx="11" cy="11" r="8" />
        <line x1="21" y1="21" x2="16.65" y2="16.65" strokeLinecap="round" />
      </svg>
    ),
  },
  {
    title: "2. Contract",
    desc: "Send a partnership request. Agree on compensation, criteria, caps, and return rules, all inside the platform.",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" strokeLinecap="round" strokeLinejoin="round" />
        <polyline points="14 2 14 8 20 8" />
      </svg>
    ),
  },
  {
    title: "3. Trade",
    desc: "Leads start flowing on the contract terms. Billing, returns, and rev share all handled automatically.",
    icon: (
      <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" strokeLinecap="round" strokeLinejoin="round" />
        <polyline points="22 4 12 14.01 9 11.01" />
      </svg>
    ),
  },
];

export default function ExchangePage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[72px] sm:pb-14">
        <div className={containerClass}>
          <Eyebrow>Exchange marketplace</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Supply meets demand.
            <br />
            <span className="text-jade-500">On contract terms.</span>
          </h1>
          <p className="mx-auto mb-7 mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Browse vetted publishers and buyers across 30+ verticals. Send a partnership request, agree on a contract,
            start trading.
          </p>
          <QuoteButton xl>Join the Exchange</QuoteButton>
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Live listings</Eyebrow>
            <H2>Buyers looking for supply.</H2>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
            {listings.map((l) => (
              <XCard key={l.name} {...l} />
            ))}
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>How the exchange works</Eyebrow>
            <H2>Partnership to first lead in a day.</H2>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            {steps.map((s) => (
              <div key={s.title} className="rounded-xl border border-gray-100 bg-white p-6">
                <div className="mb-4 flex h-[38px] w-[38px] items-center justify-center rounded-[9px] border border-jade-100 bg-jade-50 text-jade-500">
                  {s.icon}
                </div>
                <div className="mb-1.5 text-[15px] font-bold text-gray-800">{s.title}</div>
                <div className="text-xs leading-relaxed text-gray-400">{s.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Verticals</Eyebrow>
            <H2>30+ industries and counting.</H2>
          </div>
          <div className="flex flex-wrap justify-center gap-2">
            {verticals.map((v) => (
              <Pill key={v}>{v}</Pill>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
