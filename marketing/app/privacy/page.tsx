import type { Metadata } from "next";
import { containerClass } from "@/components/ui";
import { pageMeta } from "@/lib/metadata";

export const metadata: Metadata = pageMeta({
  title: "Privacy Policy",
  description: "Privacy Policy for Leadrula, the lead distribution platform operated by Affiniti.",
  path: "/privacy",
});

const sections = [
  {
    title: "1. Who we are",
    body: `Leadrula ("Leadrula," "we," "us," or "our") is a lead distribution platform operated by Affiniti. This Privacy Policy explains how we collect, use, disclose, and protect information when you visit leadrula.com, use our application at app.leadrula.com, or contact us through our website forms.`,
  },
  {
    title: "2. Information we collect",
    body: `We may collect: (a) contact and account information such as name, email address, phone number, company name, and role; (b) lead and business data you submit through the platform as a publisher or buyer; (c) usage and log data including IP address, browser type, pages viewed, and timestamps; (d) communications you send us, including quote requests and support messages; and (e) payment and billing information processed through our third-party payment providers.`,
  },
  {
    title: "3. How we use information",
    body: `We use information to provide and operate the Leadrula platform, process quote and demo requests, authenticate users, route and distribute leads according to your configuration, calculate billing and rev-share settlements, send transactional emails, improve our services, comply with legal obligations, and protect against fraud and abuse.`,
  },
  {
    title: "4. How we share information",
    body: `We share information with: (a) other users on the platform as required by your contracts and collaboration settings (for example, lead data shared between a publisher and buyer); (b) service providers who assist with hosting, email delivery, analytics, payment processing, and customer support, under contractual confidentiality obligations; (c) professional advisors and authorities when required by law; and (d) in connection with a merger, acquisition, or sale of assets, with notice where required.`,
  },
  {
    title: "5. Cookies and analytics",
    body: `We use cookies and similar technologies to maintain sessions, remember preferences, and understand how our website and application are used. You can control cookies through your browser settings. Disabling cookies may affect certain features.`,
  },
  {
    title: "6. Data retention",
    body: `We retain personal information for as long as needed to provide the services, fulfill the purposes described in this policy, comply with legal obligations, resolve disputes, and enforce agreements. Lead and activity data may be retained according to your account settings and applicable contract requirements.`,
  },
  {
    title: "7. Security",
    body: `We implement administrative, technical, and organizational measures designed to protect information against unauthorized access, loss, or alteration. No method of transmission or storage is completely secure, and we cannot guarantee absolute security.`,
  },
  {
    title: "8. Your rights",
    body: `Depending on your location, you may have rights to access, correct, delete, or restrict processing of your personal information, or to object to certain processing. To exercise these rights, contact us at sales@leadrula.com. We will respond within a reasonable timeframe.`,
  },
  {
    title: "9. International transfers",
    body: `If you access our services from outside the United States, your information may be transferred to and processed in the United States or other countries where we or our service providers operate.`,
  },
  {
    title: "10. Children",
    body: `Leadrula is a business-to-business service not directed to individuals under 18. We do not knowingly collect personal information from children.`,
  },
  {
    title: "11. Changes",
    body: `We may update this Privacy Policy from time to time. We will post the revised policy on this page and update the effective date below. Material changes may be communicated by email or in-app notice where appropriate.`,
  },
  {
    title: "12. Contact",
    body: `Questions about this Privacy Policy may be sent to sales@leadrula.com.`,
  },
];

export default function PrivacyPage() {
  return (
    <div className="border-b border-gray-100 py-12 sm:py-[72px]">
      <div className={containerClass}>
        <div className="mx-auto max-w-[720px]">
          <h1 className="mb-2 text-3xl font-extrabold tracking-tight text-gray-800">Privacy Policy</h1>
          <p className="mb-8 text-sm text-gray-400">Effective date: July 3, 2026</p>
          <p className="mb-8 text-sm leading-relaxed text-gray-600">
            This policy applies to Leadrula, operated by Affiniti. It is provided as a starting point for a SaaS lead
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
