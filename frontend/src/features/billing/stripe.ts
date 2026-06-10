import { loadStripe, type Stripe } from "@stripe/stripe-js";

const platformPublishableKey = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as string | undefined;

export const stripeConfigured = Boolean(platformPublishableKey);

export const stripePromise = platformPublishableKey ? loadStripe(platformPublishableKey) : null;

const stripeByKey = new Map<string, Promise<Stripe | null>>();

export function loadStripeForKey(publishableKey: string): Promise<Stripe | null> {
  if (!stripeByKey.has(publishableKey)) {
    stripeByKey.set(publishableKey, loadStripe(publishableKey));
  }
  return stripeByKey.get(publishableKey)!;
}

export function stripePromiseForIntent(publishableKey?: string): Promise<Stripe | null> | null {
  if (publishableKey) return loadStripeForKey(publishableKey);
  return stripePromise;
}

export function isStripeAvailable(publishableKey?: string): boolean {
  return Boolean(publishableKey || stripeConfigured);
}
