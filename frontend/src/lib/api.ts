import axios, { AxiosError } from "axios";
import { useAuthStore } from "@/store/authStore";
import { toast } from "@/store/toastStore";

function normalizeBaseURL(raw: string): string {
  const trimmed = raw.replace(/\/$/, "");
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  if (trimmed.startsWith("localhost") || trimmed.startsWith("127.0.0.1")) {
    return `http://${trimmed}`;
  }
  return `https://${trimmed}`;
}

const baseURL = normalizeBaseURL(import.meta.env.VITE_API_URL || "http://localhost:8080");

export const api = axios.create({ baseURL });

api.interceptors.request.use((cfg) => {
  const t = useAuthStore.getState().accessToken;
  if (t) cfg.headers.Authorization = `Bearer ${t}`;
  return cfg;
});

let refreshing: Promise<string | null> | null = null;

async function tryRefresh(): Promise<string | null> {
  const refresh = useAuthStore.getState().refreshToken;
  if (!refresh) return null;
  try {
    const res = await axios.post(`${baseURL}/auth/refresh`, { refresh });
    const { access, refresh: newRefresh } = res.data.data;
    useAuthStore.getState().setTokens(access, newRefresh);
    return access;
  } catch {
    useAuthStore.getState().logout();
    return null;
  }
}

api.interceptors.response.use(
  (r) => r,
  async (err: AxiosError) => {
    const original = err.config as (typeof err.config & { _retry?: boolean }) | undefined;
    if (err.response?.status === 401 && original && !original._retry) {
      original._retry = true;
      const imp = useAuthStore.getState().impersonation;
      if (imp) {
        useAuthStore.getState().endImpersonation();
        toast.error("Collaboration access revoked");
        return Promise.reject(err);
      }
      const sw = useAuthStore.getState().switchSession;
      if (sw) {
        useAuthStore.getState().endSwitch();
        toast.error("Switched session expired");
        return Promise.reject(err);
      }
      if (!refreshing) refreshing = tryRefresh();
      const newToken = await refreshing;
      refreshing = null;
      if (newToken) {
        original.headers = original.headers ?? {};
        original.headers.Authorization = `Bearer ${newToken}`;
        return api(original);
      }
    }
    return Promise.reject(err);
  }
);

export interface ApiErrorShape {
  code: string;
  message: string;
}

export class ApiError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export function apiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err;
  if (axios.isAxiosError(err)) {
    const status = err.response?.status ?? 0;
    const e = (err.response?.data as { error?: ApiErrorShape } | undefined)?.error;
    return new ApiError(status, e?.code ?? "error", e?.message ?? err.message);
  }
  return new ApiError(0, "error", "unexpected error");
}

export function isInviteEmailError(err: unknown): boolean {
  const e = apiError(err);
  return e.code === "service_unavailable" && e.message.toLowerCase().includes("invite email");
}

export function errorMessage(err: unknown): string {
  const e = apiError(err);

  if (e.message && e.message !== "unexpected error" && e.message !== "Network Error") {
    return e.message.charAt(0).toUpperCase() + e.message.slice(1);
  }

  if (e.status === 0) return "Can't reach the server. Check your connection.";
  switch (e.code) {
    case "validation_error":
      return "Check your input and try again.";
    case "conflict":
      return "That already exists.";
    case "not_found":
      return "Not found.";
    case "forbidden":
      return "You don't have permission to do that.";
    case "insufficient_balance":
      return "Insufficient balance.";
    case "business_rule":
      return "This action isn't allowed.";
    case "service_unavailable":
      return e.message.charAt(0).toUpperCase() + e.message.slice(1);
    case "internal":
    default:
      return "Something went wrong. Please try again.";
  }
}

export function ns(): string {
  const user = useAuthStore.getState().user;
  if (user?.account_type === "publisher") return "/publisher";
  if (user?.account_type === "platform") return "/platform";
  return "/buyer";
}

export async function get<T>(path: string): Promise<T> {
  try {
    const res = await api.get(path);
    return res.data.data as T;
  } catch (e) {
    throw apiError(e);
  }
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
  try {
    const res = await api.post(path, body);
    return res.data.data as T;
  } catch (e) {
    throw apiError(e);
  }
}

export async function postForm<T>(path: string, form: FormData): Promise<T> {
  try {
    const res = await api.post(path, form);
    return res.data.data as T;
  } catch (e) {
    throw apiError(e);
  }
}

export async function patch<T>(path: string, body?: unknown): Promise<T> {
  try {
    const res = await api.patch(path, body);
    return res.data.data as T;
  } catch (e) {
    throw apiError(e);
  }
}

export async function del<T>(path: string): Promise<T> {
  try {
    const res = await api.delete(path);
    return res.data.data as T;
  } catch (e) {
    throw apiError(e);
  }
}
