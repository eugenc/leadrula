import { loadStripe } from "@stripe/stripe-js";

const publishableKey = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY as string | undefined;

export const stripeConfigured = Boolean(publishableKey);

export const stripePromise = publishableKey ? loadStripe(publishableKey) : null;
