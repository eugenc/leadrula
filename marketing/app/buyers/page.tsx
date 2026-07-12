import type { Metadata } from "next";
import { pageMeta } from "@/lib/metadata";
import { containerClass, Dive, Eyebrow, FeatCard, H2, Mock, MockCard, QuoteButton, StatBar } from "@/components/ui";

export const metadata: Metadata = pageMeta({
  title: "For Buyers — Qualified Leads & Publisher Support",
  description:
    "Leads arrive pre-qualified, pre-booked, and in your CRM — with a publisher who can see where deals get stuck and help move them.",
  path: "/buyers",
});

const toolkit = [
  ["Kanban pipelines", "Custom stages, drag and drop"],
  ["Calendar", "Booked appointments in one view"],
  ["Publisher directory", "Manage supply relationships"],
  ["Routes + API keys", "Push leads to your own stack"],
  ["Returns + disputes", "Fair process, clean records"],
  ["Wallet billing", "Top-up, statements, no surprises"],
  ["Call handling", "Inbound routing, duration billing"],
  ["Delivery logs", "Every lead, fully traceable"],
];

export default function BuyersPage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[72px] sm:pb-14">
        <div className={containerClass}>
          <Eyebrow>For Buyers</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Qualified leads, delivered.
            <br />
            <span className="text-jade-500">Publisher in your corner.</span>
          </h1>
          <p className="mx-auto mb-7 mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Data, appointments, and inbound calls arrive pre-qualified and in your CRM, with a publisher who can see
            where deals get stuck and help move them.
          </p>
          <QuoteButton xl />
          <div className="mx-auto max-w-[720px]">
            <StatBar
              stats={[
                ["<2s", "Delivery time"],
                ["1-click", "Lead returns"],
                ["0", "Bad data charges"],
                ["100%", "Contract-matched leads"],
              ]}
            />
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100">
        <div className={containerClass}>
          <Dive
            title="Data, appointments, and calls in one CRM."
            desc="Every lead type lands where you work: a card on your Kanban board, a slot on your calendar, or a live call to your team, or pushed to the CRM you already run."
            items={[
              { label: "Data leads", text: "as cards on a multi-pipeline Kanban board" },
              { label: "Appointments", text: "booked slots synced to your calendar" },
              { label: "Calls", text: "live inbound routed to your team, billed by duration" },
            ]}
            mock={
              <Mock>
                <MockCard title="New: 12 leads" badge="Today" badgeVariant="info" sub="8 with booked appointments" />
                <MockCard title="Contacted: 34 leads" badge="Pipeline" badgeVariant="neutral" sub="Avg. 2.1 days in stage" />
                <MockCard title="Closed Won: 9 deals" badge="$41,200" sub="This month · Rev share settled" />
              </Mock>
            }
          />

          <Dive
            flip
            title="Only pay for leads that are real."
            desc="Contract criteria filter what reaches you. Anything that slips through gets returned in one click, on the return window you agreed to."
            items={[
              { label: "Contract filters", text: "leads outside your criteria never arrive" },
              { label: "One-click returns", text: "return with a reason, get credited, move on" },
              { label: "Wallet billing", text: "top up, get charged per lead, full statement anytime" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="Lead #8812 - wrong number"
                  badge="Return filed"
                  badgeVariant="warn"
                  sub="Auto-refunded to wallet · Publisher notified"
                />
                <MockCard title="Wallet balance" badge="$2,340" sub="Auto top-up at $500 · Charged per lead received" />
              </Mock>
            }
          />

          <Dive
            title="Your publisher helps you close."
            desc="Grant shared pipeline access and your publisher sees where deals are stuck, flagging context you'd never get from a lead file."
            items={[
              { label: "Access is yours to control", text: "you grant it, you can revoke it" },
              { label: "Flags that matter", text: 'real context like "best time to call" or "already has quotes"' },
              { label: "Full transparency", text: "every touch logged, nothing happens behind your back" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="Jane Doe - Solar, Qualified"
                  badge="Shared"
                  badgeVariant="info"
                  sub='Publisher: "Spouse is decision maker, call together"'
                />
                <MockCard
                  title="Mark T. - stuck 6 days"
                  badge="Flagged"
                  badgeVariant="warn"
                  sub="Publisher offered warm re-engagement email"
                />
              </Mock>
            }
          />
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Buyer toolkit</Eyebrow>
            <H2>Everything a lead buyer needs.</H2>
          </div>
          <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-4">
            {toolkit.map(([t, d]) => (
              <FeatCard key={t} title={t} desc={d} />
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
