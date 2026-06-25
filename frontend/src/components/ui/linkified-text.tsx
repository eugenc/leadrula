import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const URL_RE = /(https?:\/\/[^\s]+)/g;
const TRAILING_PUNCT = /[.,;:!?)]+$/;

function splitUrl(raw: string): { href: string; trailing: string } {
  const trailing = raw.match(TRAILING_PUNCT)?.[0] ?? "";
  return { href: trailing ? raw.slice(0, -trailing.length) : raw, trailing };
}

const BARE_DOMAIN_RE = /^[\w-]+(?:\.[\w-]+)+(\/\S*)?$/i;

export function urlHrefFromValue(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (/^https?:\/\//i.test(trimmed)) {
    return splitUrl(trimmed).href;
  }
  if (BARE_DOMAIN_RE.test(trimmed)) {
    return `https://${splitUrl(trimmed).href}`;
  }
  return null;
}

export function LinkifiedText({ text, className }: { text: string; className?: string }) {
  const parts = text.split(URL_RE);
  const nodes: ReactNode[] = [];

  for (let i = 0; i < parts.length; i++) {
    const part = parts[i];
    if (!part) continue;
    if (/^https?:\/\//.test(part)) {
      const { href, trailing } = splitUrl(part);
      nodes.push(
        <a
          key={i}
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          className="break-all text-jade-600 hover:underline"
        >
          {href}
        </a>
      );
      if (trailing) nodes.push(trailing);
    } else {
      nodes.push(part);
    }
  }

  return <span className={cn(className)}>{nodes}</span>;
}
