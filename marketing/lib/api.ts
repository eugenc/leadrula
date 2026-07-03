import { apiUrl } from "./env";

export type QuotePayload = {
  full_name: string;
  email: string;
  phone?: string;
  role: string;
  monthly_volume: string;
  verticals?: string;
  message?: string;
  website?: string;
};

export async function submitQuote(payload: QuotePayload): Promise<void> {
  const res = await fetch(`${apiUrl()}/public/quote`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    let msg = "Something went wrong. Please try again.";
    try {
      const data = (await res.json()) as { error?: { message?: string }; message?: string };
      msg = data.error?.message || data.message || msg;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
}
