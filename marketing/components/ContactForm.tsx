"use client";

import { useState, type FormEvent } from "react";
import { submitQuote } from "@/lib/api";

const inputCls =
  "mb-4 h-[38px] w-full rounded-lg border border-gray-200 bg-white px-3 text-[13px] text-gray-800 focus:border-jade-400 focus:outline-none focus:ring-[3px] focus:ring-jade-100";
const labelCls = "mb-1.5 block text-xs font-semibold text-gray-700";

export function ContactForm() {
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    const fd = new FormData(e.currentTarget);

    if (String(fd.get("website") || "").trim()) {
      setSubmitted(true);
      return;
    }

    const full_name = String(fd.get("full_name") || "").trim();
    const email = String(fd.get("email") || "").trim();
    if (!full_name || !email) {
      setError("Name and work email are required.");
      return;
    }

    setLoading(true);
    try {
      await submitQuote({
        full_name,
        email,
        phone: String(fd.get("phone") || "").trim(),
        role: String(fd.get("role") || "Publisher"),
        monthly_volume: String(fd.get("monthly_volume") || "Under 2,500"),
        verticals: String(fd.get("verticals") || "").trim(),
        message: String(fd.get("message") || "").trim(),
        website: String(fd.get("website") || "").trim(),
      });
      setSubmitted(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setLoading(false);
    }
  }

  if (submitted) {
    return (
      <div className="py-10 text-center">
        <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full border border-jade-200 bg-jade-50">
          <svg
            viewBox="0 0 24 24"
            className="h-6 w-6 stroke-jade-500"
            fill="none"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <polyline points="20 6 9 17 4 12" />
          </svg>
        </div>
        <div className="mb-1 text-base font-bold text-gray-800">Request received</div>
        <div className="text-[13px] text-gray-400">We&apos;ll be in touch within one business day.</div>
      </div>
    );
  }

  return (
    <form onSubmit={onSubmit}>
      <input type="text" name="website" tabIndex={-1} autoComplete="off" className="hidden" aria-hidden="true" />

      <label className={labelCls} htmlFor="full_name">
        Full name
      </label>
      <input id="full_name" name="full_name" className={inputCls} type="text" placeholder="Jane Doe" required />

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label className={labelCls} htmlFor="email">
            Work email
          </label>
          <input id="email" name="email" className={inputCls} type="email" placeholder="jane@company.com" required />
        </div>
        <div>
          <label className={labelCls} htmlFor="phone">
            Phone
          </label>
          <input id="phone" name="phone" className={inputCls} type="tel" placeholder="+1 ..." />
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label className={labelCls} htmlFor="role">
            I am a
          </label>
          <select id="role" name="role" className={inputCls} defaultValue="Publisher">
            <option>Publisher</option>
            <option>Buyer</option>
            <option>Both / Network</option>
          </select>
        </div>
        <div>
          <label className={labelCls} htmlFor="monthly_volume">
            Monthly lead volume
          </label>
          <select id="monthly_volume" name="monthly_volume" className={inputCls} defaultValue="Under 2,500">
            <option>Under 2,500</option>
            <option>2,500 to 15,000</option>
            <option>15,000+</option>
          </select>
        </div>
      </div>

      <label className={labelCls} htmlFor="verticals">
        Verticals
      </label>
      <input id="verticals" name="verticals" className={inputCls} type="text" placeholder="Solar, HVAC, Mortgage..." />

      <label className={labelCls} htmlFor="message">
        Anything else?
      </label>
      <textarea
        id="message"
        name="message"
        className="mb-4 min-h-[96px] w-full resize-y rounded-lg border border-gray-200 bg-white p-3 text-[13px] text-gray-800 focus:border-jade-400 focus:outline-none focus:ring-[3px] focus:ring-jade-100"
        placeholder="Current setup, buyers, timelines..."
      />

      {error && <p className="mb-3 text-xs text-red-600">{error}</p>}

      <button
        type="submit"
        disabled={loading}
        className="h-[42px] w-full rounded-lg bg-jade-500 text-sm font-semibold text-white transition-colors hover:bg-jade-600 disabled:opacity-60"
      >
        {loading ? "Sending..." : "Request a Quote"}
      </button>
      <p className="mt-3 text-center text-[11px] text-gray-300">Response within one business day. No spam, ever.</p>
    </form>
  );
}
