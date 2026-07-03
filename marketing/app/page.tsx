import Link from "next/link";
import type { Metadata } from "next";
import { homeMetadata } from "@/lib/metadata";
import {
  CheckItem,
  containerClass,
  Eyebrow,
  H2,
  QuoteButton,
  StatBar,
  Sub,
  XCard,
} from "@/components/ui";

export const metadata: Metadata = homeMetadata;

function RoleCard({
  icon,
  title,
  desc,
  items,
}: {
  icon: React.ReactNode;
  title: string;
  desc: string;
  items: string[];
}) {
  return (
    <div className="rounded-xl border border-gray-100 bg-white p-6 transition-all hover:border-jade-300 hover:shadow-lg hover:shadow-jade-500/10">
      <div className="mb-4 flex h-[38px] w-[38px] items-center justify-center rounded-[9px] border border-jade-100 bg-jade-50 text-jade-500">
        {icon}
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

const flowSteps = [
  ["Source", "API, forms, or CSV. Tagged by source."],
  ["Qualify", "Auto-reject dupes and bad data."],
  ["Book", "Appointments from the pipeline."],
  ["Distribute", "Routes, targets, branches, caps."],
  ["Collaborate", "Work buyer pipelines together."],
  ["Settle", "Rev share calculates on close."],
];

const quotes = [
  [
    "MK",
    "Marcus K.",
    "Publisher, Home Services",
    "We finally know which leads actually close. Rev share went from a monthly argument to a line item that settles itself.",
  ],
  [
    "SR",
    "Sara R.",
    "Buyer, Solar",
    "Leads arrive pre-qualified with appointments booked. My closers stopped chasing bad numbers and started closing.",
  ],
  [
    "DL",
    "Dmitri L.",
    "Buyer, Mortgage",
    "The shared pipeline changed everything. Our publisher flags stuck deals before they die. It's a real partnership now.",
  ],
];

export default function HomePage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[84px] sm:pb-[68px]">
        <div className={containerClass}>
          <div className="mb-[26px] inline-flex items-center gap-1.5 rounded-full border border-jade-200 bg-jade-50 px-[11px] py-1 text-[11px] font-medium text-jade-700">
            <div className="h-[5px] w-[5px] rounded-full bg-jade-500" />
            Lead distribution + exchange marketplace
          </div>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.8px] text-gray-800 sm:text-5xl">
            Every lead. Every buyer.
            <br />
            <span className="text-jade-500">One pipeline.</span>
          </h1>
          <p className="mx-auto mb-9 mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Leadrula is the lead distribution platform where publishers source, qualify, and route leads to buyers,
            and stay in the deal until it closes.
          </p>
          <div className="mb-[18px] flex flex-col items-center justify-center gap-2.5 sm:flex-row">
            <QuoteButton xl />
            <Link
              href="/platform"
              className="inline-flex h-[42px] items-center rounded-lg border border-gray-100 px-6 text-sm text-gray-600 hover:bg-gray-50"
            >
              See the platform →
            </Link>
          </div>
          <div className="text-[11px] text-gray-300">No contracts. No feature gatekeeping. Scale as you grow.</div>
          <StatBar
            stats={[
              ["<2s", "Lead delivery"],
              ["30+", "Verticals"],
              ["99.9%", "Uptime"],
              ["100%", "Rev share on close"],
            ]}
          />
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Solutions</Eyebrow>
            <H2>Built for both sides of the lead.</H2>
            <Sub center>
              Whether you sell leads, buy them, or broker between the two, one platform runs it all.
            </Sub>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <RoleCard
              icon={
                <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
                  <path d="M8 9l4-4 4 4M12 5v14" strokeLinecap="round" strokeLinejoin="round" />
                  <path d="M21 19H3" />
                </svg>
              }
              title="Publishers"
              desc="Source leads from any channel, qualify them, and distribute to your buyer network on your rules."
              items={[
                "Inbound API + intake review queue",
                "Routing rules, targets and branches",
                "Contracts, caps and rev share",
              ]}
            />
            <RoleCard
              icon={
                <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
                  <rect x="3" y="3" width="7" height="7" rx="1" />
                  <rect x="14" y="3" width="7" height="7" rx="1" />
                  <rect x="3" y="14" width="7" height="7" rx="1" />
                  <rect x="14" y="14" width="7" height="7" rx="1" />
                </svg>
              }
              title="Buyers"
              desc="Receive qualified, pre-booked leads in your own Kanban CRM, or pushed straight to the stack you already use."
              items={[
                "Pipeline board + calendar",
                "One-click lead returns and disputes",
                "Wallet billing, pay per lead received",
              ]}
            />
            <RoleCard
              icon={
                <svg viewBox="0 0 24 24" className="h-[18px] w-[18px]" fill="none" stroke="currentColor" strokeWidth={1.75}>
                  <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" strokeLinecap="round" strokeLinejoin="round" />
                  <circle cx="9" cy="7" r="4" />
                  <path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
                </svg>
              }
              title="Partnerships"
              desc="Publishers and buyers collaborate inside shared pipelines, with full activity logs and settlement on close."
              items={[
                "Shared pipeline access per buyer",
                "Collaboration requests + activity log",
                "Automatic rev share settlement",
              ]}
            />
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>How it works</Eyebrow>
            <H2>From inbound to closed. Six steps.</H2>
          </div>
          <div className="grid grid-cols-1 overflow-hidden rounded-xl border border-gray-100 bg-white sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
            {flowSteps.map(([t, d], i) => (
              <div key={t} className="border-b border-gray-100 p-4 py-[22px] last:border-b-0 sm:border-b-0 sm:border-r xl:last:border-r-0">
                <div className="mb-3 flex h-[22px] w-[22px] items-center justify-center rounded-full bg-jade-500 text-[10px] font-bold text-white">
                  {i + 1}
                </div>
                <div className="mb-1 text-xs font-bold text-gray-800">{t}</div>
                <div className="text-[11px] leading-normal text-gray-400">{d}</div>
              </div>
            ))}
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Exchange marketplace</Eyebrow>
            <H2>Not just software. A marketplace.</H2>
            <Sub center>Connect with vetted publishers and buyers across 30+ verticals.</Sub>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <XCard
              vertical="Solar"
              name="SunPath Media"
              meta="Ontario · Exclusive · 200/mo"
              rows={[
                ["Payout", "Rev share 20%"],
                ["Delivery", "Real-time API"],
                ["Acceptance", "94%"],
              ]}
            />
            <XCard
              vertical="Insurance"
              name="PolicyBridge"
              meta="North America · Multi-sell · 500/mo"
              rows={[
                ["Payout", "$28 flat/lead"],
                ["Delivery", "Ping-post"],
                ["Acceptance", "88%"],
              ]}
            />
            <XCard
              vertical="Mortgage"
              name="LendSource Partners"
              meta="US · Exclusive · 150/mo"
              rows={[
                ["Payout", "Rev share 15%"],
                ["Delivery", "CRM direct"],
                ["Acceptance", "91%"],
              ]}
            />
          </div>
          <div className="mt-6 text-center">
            <Link
              href="/exchange"
              className="inline-flex h-8 items-center rounded-md border border-gray-100 px-3.5 text-[13px] font-medium text-gray-600 hover:bg-gray-50"
            >
              Browse the exchange →
            </Link>
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>What partners say</Eyebrow>
            <H2>Trusted on both sides of the deal.</H2>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            {quotes.map(([av, name, role, text]) => (
              <div key={name} className="rounded-lg border border-gray-100 bg-white p-[22px]">
                <div className="mb-3 text-xs tracking-[2px] text-jade-500">★★★★★</div>
                <div className="mb-4 text-[12.5px] leading-relaxed text-gray-600">&ldquo;{text}&rdquo;</div>
                <div className="flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-full bg-jade-500 text-[10px] font-bold text-white">
                    {av}
                  </div>
                  <div>
                    <div className="text-[11px] font-bold text-gray-800">{name}</div>
                    <div className="text-[10px] text-gray-300">{role}</div>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
