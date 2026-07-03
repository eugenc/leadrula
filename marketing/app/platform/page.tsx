import type { Metadata } from "next";
import { pageMeta } from "@/lib/metadata";
import {
  containerClass,
  Dive,
  Eyebrow,
  FeatCard,
  H2,
  Mock,
  MockCard,
  Pill,
  QuoteButton,
} from "@/components/ui";

export const metadata: Metadata = pageMeta({
  title: "Platform — Intake, Routing, Contracts & Rev Share",
  description:
    "Everything between the lead form and the closed deal: intake, routing, contracts, collaboration, and settlement on one platform.",
  path: "/platform",
});

const features = [
  ["Inbound API", "API keys, sources, custom fields"],
  ["Auto-qualify", "Dupe check + rejection rules"],
  ["Kanban CRM", "Multi-pipeline lead boards"],
  ["Calendar", "Appointments from pipeline"],
  ["Smart routing", "Targets, branches, field maps"],
  ["Contracts", "Caps, criteria, return rules"],
  ["Collaboration", "Shared pipelines + comments"],
  ["Rev share", "Auto settlement on close"],
  ["Wallet billing", "Balance top-up, per-lead charge"],
  ["Returns", "Disputes + re-distribution"],
  ["Teams + roles", "Admin controls, invites, audit"],
  ["Activity log", "Every action, fully traceable"],
];

const integrations = [
  "GoHighLevel",
  "HubSpot",
  "Salesforce",
  "Pipedrive",
  "Zapier",
  "n8n",
  "Twilio",
  "Webhooks",
  "REST API",
  "CSV import",
];

export default function PlatformPage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[72px] sm:pb-14">
        <div className={containerClass}>
          <Eyebrow>Platform</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Distribution engine.
            <br />
            <span className="text-jade-500">Buyer CRM. One system.</span>
          </h1>
          <p className="mx-auto mb-7 mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Everything between the lead form and the closed deal: intake, routing, contracts, collaboration, and
            settlement.
          </p>
          <QuoteButton xl />
        </div>
      </div>

      <div className="border-b border-gray-100">
        <div className={containerClass}>
          <Dive
            id="intake"
            eyebrow="Intake"
            title="Qualification that filters before it costs you."
            desc="Every lead hits the intake log with full provenance. Bad data never reaches a buyer."
            items={[
              { label: "Auto-qualification", text: "duplicates and out-of-spec leads rejected on entry" },
              { label: "Review queue", text: "borderline leads held for manual approval" },
              { label: "Custom fields", text: "capture any vertical-specific data point" },
              { label: "Intake log", text: "full log of every lead received, accepted, or rejected" },
            ]}
            mock={
              <Mock>
                <div className="overflow-x-auto rounded-lg border border-gray-100 bg-white p-4 font-mono text-[10.5px] leading-relaxed text-gray-600">
                  <span className="text-jade-600">POST</span> /api/v1/leads
                  <br />
                  {"{"}
                  <br />
                  &nbsp;&nbsp;<span className="text-info">&quot;source&quot;</span>:{" "}
                  <span className="text-info">&quot;solar_ontario_q2&quot;</span>,<br />
                  &nbsp;&nbsp;<span className="text-info">&quot;first_name&quot;</span>:{" "}
                  <span className="text-info">&quot;Jane&quot;</span>,{" "}
                  <span className="text-info">&quot;phone&quot;</span>:{" "}
                  <span className="text-info">&quot;+1...&quot;</span>,<br />
                  &nbsp;&nbsp;<span className="text-info">&quot;custom&quot;</span>: {"{"}{" "}
                  <span className="text-info">&quot;utility&quot;</span>:{" "}
                  <span className="text-info">&quot;Hydro One&quot;</span> {"}"}
                  <br />
                  {"}"}
                  <br />
                  <span className="text-gray-300">→ 202</span> {"{"}{" "}
                  <span className="text-info">&quot;status&quot;</span>:{" "}
                  <span className="text-jade-600">&quot;distributed&quot;</span> {"}"}
                </div>
              </Mock>
            }
          />

          <Dive
            id="routing"
            flip
            eyebrow="Routing"
            title="Distribution that fires from the pipeline."
            desc="Routing isn't just on intake. Trigger routes from any pipeline stage: qualified, booked, or custom."
            items={[
              { label: "Targets and branches", text: "exclusive, multi-sell, weighted, or conditional" },
              { label: "Field mapping", text: "translate your fields to each buyer's spec" },
              { label: "Webhooks + integrations", text: "deliver to any CRM in real time" },
              { label: "Flexible delivery", text: "deliver as raw lead or straight into a pipeline stage" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="Solar - Ontario Exclusive"
                  badge="Active"
                  sub="Trigger: stage to Qualified · Target: SunPath · Cap 200/mo"
                  bar={72}
                />
                <MockCard
                  title="Insurance - Multi-sell"
                  badge="Active"
                  sub="Branch: state = FL to PolicyBridge, else Fallback"
                  bar={45}
                />
                <MockCard
                  title="Mortgage - Webhook"
                  badge="Paused"
                  badgeVariant="neutral"
                  sub="Destination: buyer CRM via field-mapped webhook"
                />
              </Mock>
            }
          />

          <Dive
            id="contracts"
            eyebrow="Contracts"
            title="Rev share built into the contract."
            desc="Set compensation, lead criteria, caps, and return rules per buyer. Settlement happens automatically when the deal closes."
            items={[
              { label: "Flat rate or rev share", text: "per contract, per lead type" },
              { label: "Lead criteria + caps", text: "buyers only get what the contract allows" },
              { label: "Return rules + disputes", text: "clean process, clean records, both sides" },
              { label: "Contract lifecycle", text: "track signed, pending, and expired agreements" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="SunPath Media - Solar ON"
                  badge="Signed"
                  sub="Rev share 20% on Close Won · Cap 200/mo · Returns: 48h"
                />
                <MockCard
                  title="PolicyBridge - Auto Insurance"
                  badge="Pending"
                  badgeVariant="info"
                  sub="$28 flat / distributed lead · Cap 500/mo"
                />
                <MockCard
                  title="Deal #4821 closed"
                  badge="+$740 settled"
                  sub="Rev share auto-calculated · Logged for both parties"
                />
              </Mock>
            }
          />

          <Dive
            id="collab"
            flip
            eyebrow="Collaboration"
            title="Work the deal inside the buyer's pipeline."
            desc="The lead doesn't go dark after distribution. Publishers get shared visibility to help buyers close."
            items={[
              { label: "Shared pipeline access", text: "granted per buyer, revocable anytime" },
              { label: "Comments and flags", text: "on live lead cards, both directions" },
              { label: "Full activity log", text: "accountability for every deal, every touch" },
            ]}
            mock={
              <Mock>
                <MockCard
                  title="Jane Doe - Solar, Qualified"
                  badge="Shared"
                  badgeVariant="info"
                  sub='Publisher flagged: "Best time to call is after 5pm ET"'
                />
                <MockCard
                  title="Collaboration request"
                  badge="Awaiting"
                  badgeVariant="warn"
                  sub="SunPath Media invited you to share pipeline access"
                />
                <MockCard
                  title="Activity log"
                  badge="142 events"
                  badgeVariant="neutral"
                  sub="Every stage change, comment, and return, recorded"
                />
              </Mock>
            }
          />
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-10 text-center">
            <Eyebrow>Everything included</Eyebrow>
            <H2>No add-ons. No gatekeeping.</H2>
          </div>
          <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-4">
            {features.map(([t, d]) => (
              <FeatCard key={t} title={t} desc={d} />
            ))}
          </div>
        </div>
      </div>

      <div id="integrations" className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-8 text-center">
            <Eyebrow>Integrations</Eyebrow>
            <H2>Leads land where your team works.</H2>
          </div>
          <div className="flex flex-wrap justify-center gap-2">
            {integrations.map((name) => (
              <Pill key={name}>{name}</Pill>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
