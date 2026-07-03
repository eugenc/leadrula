import Link from "next/link";
import type { Metadata } from "next";
import { pageMeta } from "@/lib/metadata";
import { CheckItem, containerClass, Eyebrow, H2 } from "@/components/ui";

export const metadata: Metadata = pageMeta({
  title: "Pricing — Pay for Leads In, Never Features",
  description:
    "Every plan includes the full platform: routing, contracts, collaboration, and rev share. You're only charged for leads coming into your account.",
  path: "/pricing",
});

const plans = [
  {
    name: "Starter",
    who: "For publishers getting off the ground",
    price: "$99",
    per: "/mo",
    sub: "Up to 2,500 leads/mo",
    hot: false,
    items: ["Full platform, every feature", "Unlimited buyers + contracts", "Inbound API + 5 sources", "Email support"],
  },
  {
    name: "Growth",
    who: "For scaling publisher operations",
    price: "$299",
    per: "/mo",
    sub: "Up to 15,000 leads/mo",
    hot: true,
    items: [
      "Everything in Starter",
      "Unlimited sources + API keys",
      "Exchange marketplace listing",
      "Priority support",
    ],
  },
  {
    name: "Scale",
    who: "For networks and high volume",
    price: "Custom",
    per: "",
    sub: "Unlimited volume",
    hot: false,
    items: ["Everything in Growth", "Dedicated account manager", "Custom integrations", "SLA + onboarding"],
  },
];

const rows: [string, string, string, string][] = [
  ["Leads / month", "2,500", "15,000", "Unlimited"],
  ["Buyers & contracts", "Unlimited", "Unlimited", "Unlimited"],
  ["Routing + pipeline triggers", "✓", "✓", "✓"],
  ["Rev share settlement", "✓", "✓", "✓"],
  ["Collaboration", "✓", "✓", "✓"],
  ["Exchange listing", "-", "✓", "✓"],
  ["Dedicated manager", "-", "-", "✓"],
  ["Support", "Email", "Priority", "SLA"],
];

const faqs: [string, string][] = [
  [
    "What counts as a lead?",
    "A lead is counted when it's successfully ingested into your account via API, form, or CSV. Rejected duplicates and failed validations don't count against your quota.",
  ],
  [
    "Do buyers pay a platform fee?",
    "No. Buyers join free and pay per lead received, on the terms set in their contract with the publisher. Wallet top-up handles billing automatically.",
  ],
  [
    "How does rev share settlement work?",
    "When a buyer marks a deal Closed Won, the platform calculates the publisher's share based on the contract, logs the settlement for both parties, and includes it in the billing cycle.",
  ],
  [
    "Can I switch from another platform?",
    "Yes. CSV import maps your existing lead data to custom fields, and our team helps re-point your sources and buyer specs during onboarding.",
  ],
  [
    "Is there a contract or minimum term?",
    "No contracts, no minimums. Plans are monthly and you can change tiers as volume shifts.",
  ],
];

export default function PricingPage() {
  return (
    <>
      <div className="border-b border-gray-100 py-12 pb-10 text-center sm:py-[72px] sm:pb-14">
        <div className={containerClass}>
          <Eyebrow>Pricing</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Pay for leads in.
            <br />
            <span className="text-jade-500">Never features.</span>
          </h1>
          <p className="mx-auto mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            Every plan includes the full platform: routing, contracts, collaboration, and rev share. You&apos;re only
            charged for leads coming into your account.
          </p>
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            {plans.map((p) => (
              <div
                key={p.name}
                className={`relative rounded-xl border bg-white p-7 ${p.hot ? "border-jade-400 shadow-xl shadow-jade-500/10" : "border-gray-100"}`}
              >
                {p.hot && (
                  <div className="absolute -top-2.5 left-1/2 -translate-x-1/2 rounded-full bg-jade-500 px-3 py-0.5 text-[10px] font-bold text-white">
                    Most popular
                  </div>
                )}
                <div className="mb-1 text-[13px] font-bold text-gray-800">{p.name}</div>
                <div className="mb-4 text-[11px] text-gray-400">{p.who}</div>
                <div className="text-3xl font-extrabold tracking-tight text-gray-800">
                  {p.price}
                  <span className="text-xs font-medium tracking-normal text-gray-400">{p.per}</span>
                </div>
                <div className="mb-5 mt-1 text-[11px] text-gray-400">{p.sub}</div>
                <Link
                  href="/contact"
                  className={`mb-5 flex h-8 w-full items-center justify-center rounded-md text-[13px] font-medium ${
                    p.hot ? "bg-jade-500 text-white hover:bg-jade-600" : "border border-gray-100 text-gray-600 hover:bg-gray-50"
                  }`}
                >
                  Get a Quote
                </Link>
                <ul className="flex flex-col gap-2">
                  {p.items.map((t) => (
                    <CheckItem key={t} text={t} />
                  ))}
                </ul>
              </div>
            ))}
          </div>
          <p className="mt-5 text-center text-[11px] text-gray-300">
            Buyers join free. Pay per lead received on contract terms. No platform fee.
          </p>
        </div>
      </div>

      <div className="border-b border-gray-100 bg-gray-50 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-8 text-center">
            <Eyebrow>Compare</Eyebrow>
            <H2>Every plan gets the full platform.</H2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] border-separate border-spacing-0 overflow-hidden rounded-lg border border-gray-100 bg-white">
              <thead>
                <tr>
                  {["", "Starter", "Growth", "Scale"].map((h, i) => (
                    <th
                      key={i}
                      className="border-b border-gray-100 bg-gray-50 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500"
                    >
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map(([label, a, b, c]) => (
                  <tr key={label}>
                    <td className="border-b border-gray-100 px-4 py-[11px] text-xs font-semibold text-gray-800">{label}</td>
                    <td className="border-b border-gray-100 px-4 py-[11px] text-xs text-gray-600">{a}</td>
                    <td className="border-b border-gray-100 px-4 py-[11px] text-xs text-gray-600">{b}</td>
                    <td className="border-b border-gray-100 px-4 py-[11px] text-xs text-gray-600">{c}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div className="border-b border-gray-100 py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mb-8 text-center">
            <Eyebrow>FAQ</Eyebrow>
            <H2>Questions, answered.</H2>
          </div>
          <div className="mx-auto max-w-[640px] overflow-hidden rounded-lg border border-gray-100 bg-white">
            {faqs.map(([q, a]) => (
              <details key={q} className="group border-b border-gray-100 last:border-b-0">
                <summary className="flex cursor-pointer items-center justify-between px-5 py-4 text-[13px] font-semibold text-gray-800 [list-style:none] after:text-base after:text-jade-500 after:content-['+'] group-open:after:content-['–']">
                  {q}
                </summary>
                <p className="max-w-[640px] px-5 pb-4 text-[12.5px] leading-relaxed text-gray-400">{a}</p>
              </details>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
