import { homePath } from "@/lib/homePath";

// Suffixes that differ between the publisher (/p) and buyer (/b) route trees.
const PUBLISHER_TO_BUYER: Record<string, string> = {
  log: "logs",
  buyers: "publishers",
  contracts: "contract",
  routing: "routes",
};
const BUYER_TO_PUBLISHER: Record<string, string> = Object.fromEntries(
  Object.entries(PUBLISHER_TO_BUYER).map(([pub, buy]) => [buy, pub])
);

// Suffixes that exist only on one side and have no equivalent on the other.
const PUBLISHER_ONLY = new Set(["sources"]);
const BUYER_ONLY = new Set(["calendar"]);

export function mapAccountPath(
  pathname: string,
  search: string,
  target: "publisher" | "buyer"
): string {
  const sourcePrefix = target === "buyer" ? "/p" : "/b";
  const targetPrefix = target === "buyer" ? "/b" : "/p";

  if (pathname !== sourcePrefix && !pathname.startsWith(`${sourcePrefix}/`)) {
    return homePath(target);
  }

  const suffix = pathname.slice(sourcePrefix.length).replace(/^\//, "");
  if (!suffix) return homePath(target);

  const segments = suffix.split("/");
  const head = segments[0];

  if (target === "buyer" && PUBLISHER_ONLY.has(head)) return homePath(target);
  if (target === "publisher" && BUYER_ONLY.has(head)) return homePath(target);

  const map = target === "buyer" ? PUBLISHER_TO_BUYER : BUYER_TO_PUBLISHER;
  if (map[head]) segments[0] = map[head];

  return `${targetPrefix}/${segments.join("/")}${search}`;
}
