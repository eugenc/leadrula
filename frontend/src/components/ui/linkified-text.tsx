import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const URL_RE = /(https?:\/\/[^\s]+)/g;
const TRAILING_PUNCT = /[.,;:!?)]+$/;

function splitUrl(raw: string): { href: string; trailing: string } {
  const trailing = raw.match(TRAILING_PUNCT)?.[0] ?? "";
  return { href: trailing ? raw.slice(0, -trailing.length) : raw, trailing };
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
