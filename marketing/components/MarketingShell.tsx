"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { QuoteButton, containerClass } from "./ui";
import { appUrl } from "@/lib/env";

const navItems = [
  { href: "/platform", label: "Platform" },
  { href: "/exchange", label: "Exchange" },
  { href: "/publishers", label: "Publishers" },
  { href: "/buyers", label: "Buyers" },
  { href: "/pricing", label: "Pricing" },
];

const ctaCopy: Record<string, { h: string; p: string }> = {
  "/": {
    h: "Ready to get paid on what closes?",
    p: "Talk to us about your lead flow. We'll map it to the platform.",
  },
  "/platform": {
    h: "Ready to get paid on what closes?",
    p: "Talk to us about your lead flow. We'll map it to the platform.",
  },
  "/publishers": {
    h: "Your leads deserve a second paycheck.",
    p: "Flat rate on distribution. Rev share on close. Both, automatically.",
  },
  "/buyers": {
    h: "Stop buying blind.",
    p: "Contract-matched leads, one-click returns, and a publisher who helps you close.",
  },
  "/exchange": {
    h: "Your next best partner is already here.",
    p: "Join the exchange and start trading on terms you control.",
  },
  "/pricing": {
    h: "Not sure which plan fits?",
    p: "Tell us your volume and verticals. We'll recommend the right setup.",
  },
};

function NavLink({ href, label }: { href: string; label: string }) {
  const pathname = usePathname();
  const active = pathname === href;
  return (
    <Link
      href={href}
      className={`flex h-[30px] items-center rounded-md px-[11px] text-[13px] font-medium transition-colors ${
        active ? "bg-jade-50 text-jade-600" : "text-gray-500 hover:bg-gray-50 hover:text-gray-800"
      }`}
    >
      {label}
    </Link>
  );
}

function Nav() {
  const [open, setOpen] = useState(false);
  const signInUrl = `${appUrl()}/login`;

  return (
    <nav className="sticky top-0 z-50 border-b border-gray-100 bg-white/90 backdrop-blur-md">
      <div className={`${containerClass} flex h-[58px] items-center justify-between`}>
        <Link href="/" className="flex items-center" onClick={() => setOpen(false)}>
          <img src="/leadrula-logo-light.png" alt="Leadrula" className="h-[26px] w-auto" />
        </Link>

        <div className="hidden items-center gap-0.5 md:flex">
          {navItems.map((item) => (
            <NavLink key={item.href} href={item.href} label={item.label} />
          ))}
        </div>

        <div className="hidden items-center gap-2 md:flex">
          <a
            href={signInUrl}
            className="inline-flex h-8 items-center rounded-md border border-gray-100 px-3.5 text-[13px] font-medium text-gray-600 hover:bg-gray-50"
          >
            Sign in
          </a>
          <QuoteButton />
        </div>

        <button
          type="button"
          className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-gray-100 text-gray-600 md:hidden"
          aria-label={open ? "Close menu" : "Open menu"}
          aria-expanded={open}
          onClick={() => setOpen((v) => !v)}
        >
          {open ? (
            <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2}>
              <path d="M6 6l12 12M18 6L6 18" strokeLinecap="round" />
            </svg>
          ) : (
            <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth={2}>
              <path d="M4 7h16M4 12h16M4 17h16" strokeLinecap="round" />
            </svg>
          )}
        </button>
      </div>

      {open && (
        <div className="border-t border-gray-100 bg-white px-4 py-3 md:hidden">
          <div className="flex flex-col gap-1">
            {navItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                onClick={() => setOpen(false)}
                className="rounded-md px-3 py-2 text-[13px] font-medium text-gray-600 hover:bg-gray-50"
              >
                {item.label}
              </Link>
            ))}
          </div>
          <div className="mt-3 flex flex-col gap-2 border-t border-gray-100 pt-3">
            <a
              href={signInUrl}
              className="inline-flex h-9 items-center justify-center rounded-md border border-gray-100 text-[13px] font-medium text-gray-600"
            >
              Sign in
            </a>
            <QuoteButton />
          </div>
        </div>
      )}
    </nav>
  );
}

function CtaBand() {
  const pathname = usePathname();
  if (pathname === "/contact" || pathname === "/privacy" || pathname === "/terms") return null;
  const copy = ctaCopy[pathname] ?? ctaCopy["/"];
  return (
    <div className="bg-gradient-to-br from-jade-600 to-jade-800 py-12 text-center sm:py-[84px]">
      <div className={containerClass}>
        <h2 className="mb-3 text-2xl font-extrabold tracking-tight text-white sm:text-[34px]">{copy.h}</h2>
        <p className="mb-[30px] text-sm text-white/65">{copy.p}</p>
        <QuoteButton xl white />
      </div>
    </div>
  );
}

function Footer() {
  const cols = [
    {
      h: "Platform",
      links: [
        ["Intake & Qualification", "/platform#intake"],
        ["Routing & Distribution", "/platform#routing"],
        ["Contracts & Rev Share", "/platform#contracts"],
        ["Collaboration", "/platform#collab"],
      ],
    },
    {
      h: "Solutions",
      links: [
        ["For Publishers", "/publishers"],
        ["For Buyers", "/buyers"],
        ["Exchange", "/exchange"],
        ["Integrations", "/platform#integrations"],
      ],
    },
    {
      h: "Company",
      links: [
        ["Pricing", "/pricing"],
        ["Contact", "/contact"],
        ["Privacy", "/privacy"],
        ["Terms", "/terms"],
      ],
    },
  ];
  return (
    <footer className="border-t border-gray-100 pb-7 pt-10">
      <div className={containerClass}>
        <div className="mb-8 grid grid-cols-1 gap-8 sm:grid-cols-2 lg:grid-cols-[2fr_1fr_1fr_1fr]">
          <div>
            <Link href="/">
              <img src="/leadrula-logo-light.png" alt="Leadrula" className="h-6 w-auto" />
            </Link>
            <div className="mt-3 max-w-[220px] text-xs leading-relaxed text-gray-400">
              The lead distribution platform where publishers and buyers close together.
            </div>
          </div>
          {cols.map((col) => (
            <div key={col.h}>
              <div className="mb-3 text-[11px] font-bold uppercase tracking-wider text-gray-800">{col.h}</div>
              {col.links.map(([label, href]) => (
                <Link key={label} href={href} className="mb-2 block text-xs text-gray-400 hover:text-jade-600">
                  {label}
                </Link>
              ))}
            </div>
          ))}
        </div>
        <div className="flex flex-col items-start justify-between gap-2 border-t border-gray-100 pt-5 text-[11px] text-gray-300 sm:flex-row sm:items-center">
          <span>&copy; 2026 Leadrula. All rights reserved.</span>
          <span>An Affiniti company</span>
        </div>
      </div>
    </footer>
  );
}

export function MarketingShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-full flex-1 flex-col bg-white font-sans text-[13px] text-gray-800 antialiased">
      <Nav />
      <main className="flex-1">{children}</main>
      <CtaBand />
      <Footer />
    </div>
  );
}
