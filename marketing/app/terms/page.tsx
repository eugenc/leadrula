import type { Metadata } from "next";
import { containerClass } from "@/components/ui";
import { pageMeta } from "@/lib/metadata";

export const metadata: Metadata = pageMeta({
  title: "Terms of Service",
  description: "Terms of Service for Leadrula, the lead distribution platform operated by Affiniti.",
  path: "/terms",
});

const sections = [
  {
    title: "1. Agreement",
    body: `These Terms of Service ("Terms") govern access to and use of Leadrula, including leadrula.com, app.leadrula.com, and related services (the "Service") operated by Affiniti ("Leadrula," "we," "us," or "our"). By creating an account, accessing the Service, or submitting a quote request, you agree to these Terms.`,
  },
  {
    title: "2. The Service",
    body: `Leadrula provides software for lead intake, qualification, routing, distribution, buyer CRM, contracts, collaboration, billing, and settlement between publishers and buyers. We may modify, suspend, or discontinue features with reasonable notice where practicable.`,
  },
  {
    title: "3. Accounts",
    body: `You must provide accurate registration information and keep credentials secure. You are responsible for activity under your account. We may suspend or terminate accounts that violate these Terms, applicable law, or create security or abuse risk.`,
  },
  {
    title: "4. Acceptable use",
    body: `You agree not to: (a) use the Service for unlawful, deceptive, or abusive lead generation or sales practices; (b) transmit malware or attempt unauthorized access; (c) reverse engineer the Service except where permitted by law; (d) resell or sublicense the Service without authorization; or (e) interfere with other users' use of the Service.`,
  },
  {
    title: "5. Lead data and compliance",
    body: `Publishers and buyers are responsible for obtaining valid consent and complying with applicable telemarketing, privacy, and industry regulations (including TCPA, CAN-SPAM, state privacy laws, and sector-specific rules) for leads they ingest, distribute, or contact through the Service. Leadrula provides tools but does not guarantee lead quality or legal compliance of user data practices.`,
  },
  {
    title: "6. Fees and payment",
    body: `Publisher subscription fees, per-lead charges, wallet top-ups, and rev-share settlements are described in your plan, contracts, and billing settings. Fees are non-refundable except where required by law or explicitly stated. Buyers pay publishers according to contract terms configured on the platform.`,
  },
  {
    title: "7. Intellectual property",
    body: `Leadrula and its licensors own the Service, software, branding, and documentation. You retain ownership of your data. You grant us a limited license to host, process, and display your data solely to provide the Service.`,
  },
  {
    title: "8. Confidentiality",
    body: `Each party may receive non-public information from the other. You agree to use such information only for purposes of using the Service and to protect it with reasonable care.`,
  },
  {
    title: "9. Disclaimers",
    body: `THE SERVICE IS PROVIDED "AS IS" AND "AS AVAILABLE." TO THE MAXIMUM EXTENT PERMITTED BY LAW, WE DISCLAIM ALL WARRANTIES, EXPRESS OR IMPLIED, INCLUDING MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. WE DO NOT WARRANT UNINTERRUPTED OR ERROR-FREE OPERATION.`,
  },
  {
    title: "10. Limitation of liability",
    body: `TO THE MAXIMUM EXTENT PERMITTED BY LAW, LEADRULA AND AFFINITI WILL NOT BE LIABLE FOR INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, OR PUNITIVE DAMAGES, OR ANY LOSS OF PROFITS, REVENUE, DATA, OR GOODWILL. OUR TOTAL LIABILITY FOR ANY CLAIM ARISING OUT OF THESE TERMS OR THE SERVICE WILL NOT EXCEED THE AMOUNTS PAID BY YOU TO LEADRULA IN THE TWELVE (12) MONTHS BEFORE THE EVENT GIVING RISE TO THE CLAIM.`,
  },
  {
    title: "11. Indemnification",
    body: `You will defend and indemnify Leadrula and Affiniti against claims arising from your data, your use of the Service, your lead sourcing or contact practices, or your breach of these Terms.`,
  },
  {
    title: "12. Termination",
    body: `Either party may terminate for material breach not cured within thirty (30) days of notice. Upon termination, your right to access the Service ends. Provisions that by nature should survive will survive, including payment obligations, disclaimers, limitations of liability, and indemnification.`,
  },
  {
    title: "13. Governing law",
    body: `These Terms are governed by the laws of the State of Delaware, USA, without regard to conflict-of-law principles. Exclusive jurisdiction and venue for disputes arising under these Terms shall be in the state or federal courts located in Delaware, and each party consents to such jurisdiction.`,
  },
  {
    title: "14. Changes",
    body: `We may update these Terms by posting a revised version on this page. Continued use after the effective date constitutes acceptance of the updated Terms.`,
  },
  {
    title: "15. Contact",
    body: `Questions about these Terms may be sent to sales@leadrula.com.`,
  },
];

export default function TermsPage() {
  return (
    <div className="border-b border-gray-100 py-12 sm:py-[72px]">
      <div className={containerClass}>
        <div className="mx-auto max-w-[720px]">
          <h1 className="mb-2 text-3xl font-extrabold tracking-tight text-gray-800">Terms of Service</h1>
          <p className="mb-8 text-sm text-gray-400">Effective date: July 3, 2026</p>
          <p className="mb-8 text-sm leading-relaxed text-gray-600">
            These Terms apply to Leadrula, operated by Affiniti. They are provided as a starting point for a SaaS lead
            distribution platform and should be reviewed by qualified legal counsel before production use.
          </p>
          <div className="flex flex-col gap-8">
            {sections.map((s) => (
              <section key={s.title}>
                <h2 className="mb-2 text-base font-bold text-gray-800">{s.title}</h2>
                <p className="text-sm leading-relaxed text-gray-600">{s.body}</p>
              </section>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
