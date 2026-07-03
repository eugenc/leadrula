export function siteUrl() {
  return (process.env.NEXT_PUBLIC_SITE_URL || "https://leadrula.com").replace(/\/$/, "");
}

export function appUrl() {
  return (process.env.NEXT_PUBLIC_APP_URL || "https://app.leadrula.com").replace(/\/$/, "");
}

export function apiUrl() {
  const raw = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
  const trimmed = raw.replace(/\/$/, "");
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  if (trimmed.startsWith("localhost") || trimmed.startsWith("127.0.0.1")) {
    return `http://${trimmed}`;
  }
  return `https://${trimmed}`;
}
