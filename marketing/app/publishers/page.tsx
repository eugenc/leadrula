import type { Metadata } from "next";
import { pageMeta } from "@/lib/metadata";
import { containerClass, Dive, Eyebrow, FeatCard, H2, Mock, MockCard, QuoteButton, StatBar } from "@/components/ui";

export const metadata: Metadata = pageMeta({
  title: "For Publishers — Source, Route & Get Paid on Close",
  description:
    "Stop selling leads into a black hole. Route on your rules, watch buyer pipelines, and settle rev share automatically.",
  path: "/publishers",
});

const toolkit = [
  ["Sources + API keys", "Unlimited, individually revocable"],
  ["Intake review", "Approve borderline leads manually"],
  ["Buyer directory", "Manage your whole buyer network"],
  ["Contracts", "Caps, criteria, delivery, returns"],
  ["Routing engine", "Data, appointments, and call routing"],
  ["Own pipelines", "Qualify before you distribute"],
  ["Collaboration", "Help buyers close your leads"],
  ["Billing", "Automatic charges + settlements"],
];

export default function PublishersPage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[72px] sm:pb-14">
        <div className={containerClass}>
          <Eyebrow>For Publishers</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Source leads. Know what closes.
            <br />
            <span className="text-jade-500">Get paid on it.</span>
          </h1>
          <p className="mx-auto mb-7 mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Distribute data, appointments, and calls on your own rules, watch them move through the buyer&apos;s
            pipeline, and settle rev share automatically.
          </p>
          <QuoteButton xl />
          <div className="mx-auto max-w-[720px]">
            <StatBar
              stats={[
                ["6", "Distribution modes"],
                ["∞", "Buyers per account"],
                ["48h", "Return windows"],
                ["0", "Settlement disputes"],
              ]}
            />
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100">
        <div className={containerClass}>
          <Dive
            title="Every lead type, one intake."
            desc="Bring in data, appointments, and calls from any channel. Everything lands in one log, tagged by source and type, with your custom fields intact."
            items={[
              { label: "Data", text: "API, web forms, or CSV, with custom fields" },
              { label: "Appointments", text: "booked slots that sync to buyer calendars" },
              { label: "Calls", text: "inbound numbers routed live by RTB or static rules" },
            ]}
            mock={
              <Mock>
                <MockCard title="solar_ontario_q2" badge="1,204 leads" sub="API source · Last lead 2 min ago" />
                <MockCard title="facebook_hvac_march" badge="618 leads" sub="Webhook source · Last lead 14 min ago" />
                <MockCard
                  title="tradeshow_import.csv"
                  badge="340 leads"
                  badgeVariant="neutral"
                  sub="CSV upload · Mapped to 12 custom fields"
                />
              </Mock>
            }
          />

          <Dive
            flip
            title="Distribution on your terms."
            desc="Qualify and warm leads in your own pipeline first, then let stage changes trigger the distribution. Booked appointments are worth more than raw form fills."
            items={[
              { label: "Pipeline triggers", text: "distribute on any stage change, not just intake" },
              { label: "Every mode", text: "exclusive, multi-sell, weighted, or conditional branches" },
              { label: "Zero waste", text: "returned leads flow back and re-route automatically" },
            ]}
            mock={
              <Mock>
                <MockCard title="Qualified: distribute to SunPath" badge="Trigger" sub="Fires when a lead enters the Qualified stage" />
                <MockCard title="Booked: notify buyer + calendar" badge="Trigger" sub="Appointment synced to buyer's calendar" />
                <MockCard
                  title="Returned: re-route to fallback"
                  badge="Trigger"
                  badgeVariant="info"
                  sub="Returned leads automatically re-distributed"
                />
              </Mock>
            }
          />

          <Dive
            title="Rev share you can actually enforce."
            desc="Flat rate per lead is fine. But your best leads are worth a share of the close, and now you can see the close."
            items={[
              { label: "Flexible compensation", text: "flat, rev share, or hybrid per contract" },
              { label: "Shared visibility", text: "watch your leads move through the buyer's stages" },
              { label: "Auto settlement", text: "close event calculates your cut and logs it for both sides" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="March settlement - SunPath"
                  badge="$4,120"
                  sub="14 closed deals · 20% rev share · Auto-calculated"
                />
                <MockCard
                  title="March settlement - PolicyBridge"
                  badge="$8,404"
                  sub="318 leads · $28 flat + 3 disputes resolved"
                />
              </Mock>
            }
          />
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Publisher toolkit</Eyebrow>
            <H2>Everything a lead seller needs.</H2>
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
