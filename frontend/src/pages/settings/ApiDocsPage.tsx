import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { ChevronDown, ChevronRight, Copy } from "lucide-react";
import { apiBaseURL } from "@/lib/api";
import { useAuthStore } from "@/store/authStore";
import { PageHeader } from "@/components/layout/PageHeader";
import { PageBody } from "@/components/layout/PageBody";
import { Card } from "@/components/ui/misc";
import { IconButton } from "@/components/layout/IconButton";
import { toast } from "@/store/toastStore";
import {
  ERROR_CODES,
  OUTBOUND_TRIGGER_EVENTS,
  jwtEndpointGroups,
  publicEndpoints,
  type DocEndpoint,
  type DocGroup,
} from "@/features/api-docs/endpoints";

function prefixFromPath(pathname: string): string {
  if (pathname.startsWith("/p/")) return "/p";
  return "/b";
}

function copyText(text: string, label: string) {
  navigator.clipboard.writeText(text).then(
    () => toast.success(label),
    () => toast.error("Could not copy to clipboard")
  );
}

function MethodBadge({ method }: { method: string }) {
  return (
    <span className="rounded bg-jade-50 px-2 py-0.5 font-mono text-xs font-semibold text-jade-700">
      {method}
    </span>
  );
}

function CodeBlock({ code, copyLabel = "Copied" }: { code: string; copyLabel?: string }) {
  return (
    <div className="relative">
      <pre className="overflow-x-auto rounded-md bg-gray-50 p-3 font-mono text-xs text-gray-800">{code}</pre>
      <IconButton
        className="absolute right-2 top-2"
        aria-label="Copy"
        onClick={() => copyText(code, copyLabel)}
      >
        <Copy className="h-3.5 w-3.5" />
      </IconButton>
    </div>
  );
}

function EndpointCard({ ep, baseURL }: { ep: DocEndpoint; baseURL: string }) {
  const [open, setOpen] = useState(false);
  const fullPath = ep.path.startsWith("/") ? `${baseURL}${ep.path}` : ep.path;

  return (
    <div className="border-b border-gray-100 last:border-0">
      <button
        type="button"
        className="flex w-full items-start gap-3 px-4 py-3 text-left hover:bg-gray-50"
        onClick={() => setOpen((v) => !v)}
      >
        {open ? (
          <ChevronDown className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" />
        ) : (
          <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" />
        )}
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <MethodBadge method={ep.method} />
            <code className="break-all font-mono text-xs text-gray-800">{ep.path}</code>
          </div>
          <p className="mt-1 text-sm text-gray-500">{ep.description}</p>
          <p className="mt-0.5 text-xs text-gray-400">Auth: {ep.auth}</p>
        </div>
      </button>
      {open && (
        <div className="space-y-3 border-t border-gray-50 bg-gray-50/50 px-4 py-3 pl-11">
          <div className="flex items-center gap-2">
            <code className="flex-1 break-all font-mono text-xs text-gray-600">{fullPath}</code>
            <IconButton aria-label="Copy URL" onClick={() => copyText(fullPath, "URL copied")}>
              <Copy className="h-3.5 w-3.5" />
            </IconButton>
          </div>
          {ep.queryParams && ep.queryParams.length > 0 && (
            <div>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">Query parameters</p>
              <ul className="space-y-1 text-sm text-gray-600">
                {ep.queryParams.map((q) => (
                  <li key={q.name}>
                    <code className="text-xs text-gray-800">{q.name}</code> — {q.description}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {ep.request && <CodeBlock code={ep.request} copyLabel="Example copied" />}
          {ep.response && (
            <div>
              <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">Response</p>
              <CodeBlock code={ep.response} copyLabel="Response copied" />
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function EndpointGroup({ group }: { group: DocGroup }) {
  const [open, setOpen] = useState(group.id === "leads" || group.id === "auth");

  return (
    <Card className="overflow-hidden">
      <button
        type="button"
        className="flex w-full items-center gap-2 px-5 py-4 text-left hover:bg-gray-50"
        onClick={() => setOpen((v) => !v)}
      >
        {open ? <ChevronDown className="h-4 w-4 text-gray-400" /> : <ChevronRight className="h-4 w-4 text-gray-400" />}
        <div>
          <h3 className="text-sm font-semibold text-gray-800">{group.title}</h3>
          {group.description && <p className="text-sm text-gray-500">{group.description}</p>}
        </div>
      </button>
      {open && (
        <div className="border-t border-gray-100">
          {group.endpoints.map((ep) => (
            <EndpointCard key={`${ep.method}-${ep.path}`} ep={ep} baseURL={apiBaseURL} />
          ))}
        </div>
      )}
    </Card>
  );
}

export function ApiDocsPage() {
  const { pathname } = useLocation();
  const user = useAuthStore((s) => s.user);
  const prefix = prefixFromPath(pathname);
  const isPublisher = user?.account_type === "publisher";
  const ns = isPublisher ? "/publisher" : "/buyer";
  const webhooksPath = `${prefix}/webhooks`;

  const publicEps = publicEndpoints(apiBaseURL).filter((ep) => {
    if (isPublisher) return true;
    if (ep.path === "/api/v1/leads" && ep.method === "POST") return false;
    if (ep.path.startsWith("/api/v1/sources")) return false;
    return true;
  });

  const jwtGroups = jwtEndpointGroups(ns);

  return (
    <PageBody>
      <p className="mb-4 text-sm">
        <Link to={`${prefix}/settings/api`} className="text-jade-600 hover:underline">
          ← Back to API keys
        </Link>
      </p>

      <PageHeader title="API documentation" className="px-0 pt-0" />

      <div className="space-y-8 pb-8">
      {/* Public API */}
      <section className="space-y-4">
        <div>
          <h2 className="text-base font-semibold text-gray-800">Public API</h2>
          <p className="mt-1 text-sm text-gray-500">
            Base URL:{" "}
            <code className="rounded bg-gray-50 px-1.5 py-0.5 font-mono text-xs">{apiBaseURL}</code>
            <IconButton
              className="ml-1 inline-flex align-middle"
              aria-label="Copy base URL"
              onClick={() => copyText(apiBaseURL, "Base URL copied")}
            >
              <Copy className="h-3.5 w-3.5" />
            </IconButton>
          </p>
        </div>

        <Card className="p-5">
          <h3 className="mb-1 text-sm font-semibold text-gray-800">Overview</h3>
          <ul className="list-inside list-disc space-y-1 text-sm text-gray-600">
            <li>All requests and responses use JSON (<code className="text-xs">Content-Type: application/json</code>)</li>
            <li>Success responses: <code className="text-xs">{`{ "data": ... }`}</code></li>
            <li>Error responses: <code className="text-xs">{`{ "error": { "code", "message" } }`}</code></li>
          </ul>
        </Card>

        <Card className="p-5">
          <h3 className="mb-1 text-sm font-semibold text-gray-800">Authentication</h3>
          <p className="mb-2 text-sm text-gray-500">
            Pass your API key in the <code className="text-xs">Authorization</code> header:
          </p>
          <CodeBlock code={`Authorization: Bearer {prefix}.{secret}`} copyLabel="Header copied" />
          <p className="mt-3 text-sm text-gray-500">
            Scopes: <code className="text-xs">leads:read</code> (list/get leads),{" "}
            <code className="text-xs">leads:write</code> (ingest and update). Keys with{" "}
            <code className="text-xs">leads:write</code> may also read leads.
          </p>
          <p className="mt-2 text-sm text-gray-500">
            Generate keys in{" "}
            <Link to={`${prefix}/settings/api`} className="text-jade-600 hover:underline">
              Settings → API
            </Link>
            .
          </p>
        </Card>

        <Card className="overflow-hidden">
          <div className="border-b border-gray-100 px-5 py-3">
            <h3 className="text-sm font-semibold text-gray-800">Endpoints</h3>
          </div>
          {publicEps.map((ep) => (
            <EndpointCard key={`${ep.method}-${ep.path}`} ep={ep} baseURL={apiBaseURL} />
          ))}
        </Card>

        <Card className="p-5">
          <h3 className="mb-1 text-sm font-semibold text-gray-800">Outbound webhooks</h3>
          <p className="mb-3 text-sm text-gray-500">
            Leadrula sends HTTP requests to a configured outbound URL when trigger conditions match.
            Configure webhooks in{" "}
            <Link to={webhooksPath} className="text-jade-600 hover:underline">
              Webhooks
            </Link>
            .
          </p>
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">Trigger events</p>
          <ul className="space-y-1 text-sm text-gray-600">
            {OUTBOUND_TRIGGER_EVENTS.map((e) => (
              <li key={e.id}>
                <code className="text-xs">{e.id}</code> — {e.label}
              </li>
            ))}
          </ul>
          <p className="mt-3 text-sm text-gray-500">
            Payloads are JSON (template or field-map driven per webhook). An optional signing secret may be enabled on the webhook.
          </p>
        </Card>

        <Card className="p-5">
          <h3 className="mb-1 text-sm font-semibold text-gray-800">Error codes</h3>
          <div className="flex flex-wrap gap-2">
            {ERROR_CODES.map((code) => (
              <code key={code} className="rounded bg-gray-50 px-2 py-0.5 text-xs text-gray-700">
                {code}
              </code>
            ))}
          </div>
        </Card>
      </section>

      {/* JWT API */}
      <section className="space-y-4">
        <div>
          <h2 className="text-base font-semibold text-gray-800">Authenticated API</h2>
          <p className="mt-1 text-sm text-gray-500">
            Session-based access using JWT bearer tokens. Routes are namespaced under{" "}
            <code className="text-xs">{ns}</code> for your account type.
          </p>
        </div>

        <Card className="p-5">
          <h3 className="mb-1 text-sm font-semibold text-gray-800">Authentication flow</h3>
          <div className="space-y-3 text-sm text-gray-600">
            <div>
              <p className="mb-1 font-medium text-gray-700">1. Login</p>
              <CodeBlock
                code={`curl -s -X POST "${apiBaseURL}/auth/login" \\
  -H "Content-Type: application/json" \\
  -d '{ "email": "you@example.com", "password": "..." }'`}
              />
              <p className="mt-1 text-xs text-gray-500">Returns access and refresh tokens in the data envelope.</p>
            </div>
            <div>
              <p className="mb-1 font-medium text-gray-700">2. Authenticated requests</p>
              <CodeBlock code={`Authorization: Bearer {access_token}`} copyLabel="Header copied" />
            </div>
            <div>
              <p className="mb-1 font-medium text-gray-700">3. Refresh</p>
              <CodeBlock
                code={`curl -s -X POST "${apiBaseURL}/auth/refresh" \\
  -H "Content-Type: application/json" \\
  -d '{ "refresh": "YOUR_REFRESH_TOKEN" }'`}
              />
            </div>
          </div>
        </Card>

        <div className="space-y-3">
          {jwtGroups.map((group) => (
            <EndpointGroup key={group.id} group={group} />
          ))}
        </div>
      </section>
      </div>
    </PageBody>
  );
}
