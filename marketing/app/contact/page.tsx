import type { Metadata } from "next";
import { ContactForm } from "@/components/ContactForm";
import { containerClass, Eyebrow } from "@/components/ui";
import { pageMeta } from "@/lib/metadata";

export const metadata: Metadata = pageMeta({
  title: "Contact — Get a Quote",
  description:
    "Tell us about your lead flow. We'll map your sources, buyers, and contracts to the platform and get you a quote within one business day.",
  path: "/contact",
});

export default function ContactPage() {
  return (
    <>
      <div className="py-12 pb-0 text-center sm:py-[72px]">
        <div className={containerClass}>
          <Eyebrow>Get a Quote</Eyebrow>
          <h1 className="text-3xl font-extrabold leading-[1.06] tracking-[-1.5px] text-gray-800 sm:text-[40px]">
            Tell us about
            <br />
            <span className="text-jade-500">your lead flow.</span>
          </h1>
          <p className="mx-auto mt-[18px] max-w-[480px] text-base leading-relaxed text-gray-400">
            We&apos;ll map your sources, buyers, and contracts to the platform, and get you a quote within one business
            day.
          </p>
        </div>
      </div>

      <div className="py-12 sm:py-[72px]">
        <div className={containerClass}>
          <div className="mx-auto max-w-[520px] rounded-xl border border-gray-100 bg-white p-6 sm:p-8">
            <ContactForm />
          </div>
        </div>
      </div>
    </>
  );
}
