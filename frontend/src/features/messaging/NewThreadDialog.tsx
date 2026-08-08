import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { get, messagingAccountType } from "@/lib/api";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { useAuthStore } from "@/store/authStore";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { useUsers } from "@/features/leads/hooks";
import { usePlatformBuyers, usePlatformPublishers } from "@/features/auth/switchHooks";
import { useCreateInternalDirect, useCreateGroup, useCreateDirect } from "./hooks";

type Mode = "teammate" | "team-group" | "external";

interface PartnerRow {
  id: string;
  name: string;
  handler_id: string;
}

function matchesPartnerSearch(row: PartnerRow, q: string): boolean {
  if (!q) return true;
  const needle = q.toLowerCase();
  return row.name.toLowerCase().includes(needle) || row.handler_id.toLowerCase().includes(needle);
}

function PartnerSearchList({
  rows,
  search,
  onSearchChange,
  selected,
  onToggle,
  multiple,
  loading,
  emptyLabel,
}: {
  rows: PartnerRow[];
  search: string;
  onSearchChange: (v: string) => void;
  selected: string[];
  onToggle: (id: string) => void;
  multiple: boolean;
  loading?: boolean;
  emptyLabel: string;
}) {
  const filtered = rows.filter((r) => matchesPartnerSearch(r, search.trim()));

  return (
    <>
      <Input
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        placeholder="Search name or handler ID…"
      />
      <div className="max-h-56 overflow-y-auto rounded-md border border-gray-100">
        {loading && <p className="p-3 text-center text-xs text-gray-400">Loading…</p>}
        {!loading && filtered.length === 0 && (
          <p className="p-3 text-center text-xs text-gray-400">{emptyLabel}</p>
        )}
        {!loading &&
          filtered.map((pr) => (
            <label key={pr.id} className="flex cursor-pointer items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
              <input
                type={multiple ? "checkbox" : "radio"}
                name="partner"
                checked={selected.includes(pr.id)}
                onChange={() => onToggle(pr.id)}
              />
              <span className="min-w-0 truncate text-sm text-gray-700">{pr.name}</span>
              <span className="ml-auto shrink-0 font-mono text-[11px] text-gray-400">{pr.handler_id}</span>
            </label>
          ))}
      </div>
    </>
  );
}

export function NewThreadDialog({
  onCreated,
  onCancel,
}: {
  onCreated: (id: string) => void;
  onCancel: () => void;
}) {
  const user = useAuthStore((s) => s.user);
  const isFollower = user?.role === "follower";
  const [mode, setMode] = useState<Mode>("teammate");

  return (
    <div className="flex h-full flex-col">
      <div className="flex gap-1 border-b border-gray-100 p-2">
        <ModeTab label="Teammate" active={mode === "teammate"} onClick={() => setMode("teammate")} />
        <ModeTab label="Team group" active={mode === "team-group"} onClick={() => setMode("team-group")} />
        {!isFollower && (
          <ModeTab label="External" active={mode === "external"} onClick={() => setMode("external")} />
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {mode === "teammate" && <TeammateForm onCreated={onCreated} onCancel={onCancel} />}
        {mode === "team-group" && <TeamGroupForm onCreated={onCreated} onCancel={onCancel} />}
        {mode === "external" && <ExternalForm onCreated={onCreated} onCancel={onCancel} />}
      </div>
    </div>
  );
}

function ModeTab({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex-1 rounded-md px-2 py-1 text-xs font-medium transition-colors",
        active ? "bg-jade-50 text-jade-700" : "text-gray-500 hover:bg-gray-100"
      )}
    >
      {label}
    </button>
  );
}

function TeammateForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const { data: users } = useUsers();
  const me = useAuthStore((s) => s.user);
  const [selected, setSelected] = useState<string>("");
  const create = useCreateInternalDirect();
  const teammates = (users ?? []).filter((u) => u.public_id && u.public_id !== me?.id && u.status === "active");

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-gray-500">Start a direct message with a teammate.</p>
      <div className="max-h-72 overflow-y-auto rounded-md border border-gray-100">
        {teammates.map((u) => (
          <label key={u.public_id} className="flex cursor-pointer items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
            <input
              type="radio"
              name="teammate"
              checked={selected === u.public_id}
              onChange={() => setSelected(u.public_id!)}
            />
            <span className="text-sm text-gray-700">{u.full_name || u.email}</span>
          </label>
        ))}
        {teammates.length === 0 && <p className="p-3 text-center text-xs text-gray-400">No teammates.</p>}
      </div>
      <FormActions
        onCancel={onCancel}
        disabled={!selected || create.isPending}
        onSubmit={() =>
          create.mutate(
            { user_id: selected },
            { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
          )
        }
      />
    </div>
  );
}

function TeamGroupForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const { data: users } = useUsers();
  const me = useAuthStore((s) => s.user);
  const [title, setTitle] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const create = useCreateGroup();
  const teammates = (users ?? []).filter((u) => u.public_id && u.public_id !== me?.id && u.status === "active");

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  return (
    <div className="flex flex-col gap-2">
      <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Group name" />
      <div className="max-h-60 overflow-y-auto rounded-md border border-gray-100">
        {teammates.map((u) => (
          <label key={u.public_id} className="flex cursor-pointer items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
            <input type="checkbox" checked={selected.includes(u.public_id!)} onChange={() => toggle(u.public_id!)} />
            <span className="text-sm text-gray-700">{u.full_name || u.email}</span>
          </label>
        ))}
      </div>
      <FormActions
        onCancel={onCancel}
        disabled={!title.trim() || selected.length === 0 || create.isPending}
        onSubmit={() =>
          create.mutate(
            { title, internal: true, user_ids: selected },
            { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
          )
        }
      />
    </div>
  );
}

function ExternalForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const accountType = messagingAccountType();

  if (accountType === "platform") {
    return <PlatformExternalForm onCreated={onCreated} onCancel={onCancel} />;
  }
  if (accountType === "publisher") {
    return <PublisherExternalForm onCreated={onCreated} onCancel={onCancel} />;
  }
  if (accountType === "buyer") {
    return <BuyerExternalForm onCreated={onCreated} onCancel={onCancel} />;
  }
  return null;
}

interface BuyerPartnerRow {
  public_id: string;
  name: string;
  handler_id: string;
}

interface PartnerPublisherRow {
  id: number;
  public_id: string;
  name: string;
  handler_id: string;
}

function PublisherExternalForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const [kind, setKind] = useState<"buyer" | "publisher">("buyer");
  const [isGroup, setIsGroup] = useState(false);
  const [title, setTitle] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const createDirect = useCreateDirect();
  const createGroup = useCreateGroup();

  const { data: buyers, isLoading: buyersLoading } = useQuery({
    queryKey: ["messaging-partners", "buyers"],
    queryFn: () => get<BuyerPartnerRow[]>(`/publisher/buyers`),
  });
  const { data: partnerPubs, isLoading: pubsLoading } = useQuery({
    queryKey: ["messaging-partners", "partner-publishers"],
    queryFn: () => get<PartnerPublisherRow[]>(`/publisher/partnerships/publishers`),
  });

  const rows: PartnerRow[] =
    kind === "buyer"
      ? (buyers ?? []).map((b) => ({ id: b.public_id, name: b.name, handler_id: b.handler_id }))
      : (partnerPubs ?? []).map((p) => ({ id: p.public_id, name: p.name, handler_id: p.handler_id }));

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : isGroup ? [...s, id] : [id]));

  useEffect(() => {
    setSelected([]);
    setSearch("");
  }, [kind, isGroup]);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex gap-1">
        <ModeTab label="Direct" active={!isGroup} onClick={() => { setIsGroup(false); setSelected([]); }} />
        <ModeTab label="Group" active={isGroup} onClick={() => { setIsGroup(true); setSelected([]); }} />
      </div>
      <div className="flex gap-1">
        <ModeTab label="Buyers" active={kind === "buyer"} onClick={() => setKind("buyer")} />
        <ModeTab label="Partner publishers" active={kind === "publisher"} onClick={() => setKind("publisher")} />
      </div>
      {isGroup && <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Group name" />}
      <PartnerSearchList
        rows={rows}
        search={search}
        onSearchChange={setSearch}
        selected={selected}
        onToggle={toggle}
        multiple={isGroup}
        loading={kind === "buyer" ? buyersLoading : pubsLoading}
        emptyLabel={kind === "buyer" ? "No buyers match." : "No partner publishers match."}
      />
      <FormActions
        onCancel={onCancel}
        disabled={selected.length === 0 || (isGroup && !title.trim()) || createDirect.isPending || createGroup.isPending}
        onSubmit={() => {
          if (isGroup) {
            createGroup.mutate(
              { title, internal: false, member_ids: selected },
              { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
            );
          } else {
            createDirect.mutate(
              { recipient_account_id: selected[0], context: "general" },
              { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
            );
          }
        }}
      />
    </div>
  );
}

function BuyerExternalForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const [isGroup, setIsGroup] = useState(false);
  const [title, setTitle] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const createDirect = useCreateDirect();
  const createGroup = useCreateGroup();

  const { data: publishers, isLoading } = useQuery({
    queryKey: ["messaging-partners", "publishers"],
    queryFn: () => get<BuyerPartnerRow[]>(`/buyer/publishers`),
  });

  const rows: PartnerRow[] = (publishers ?? []).map((p) => ({
    id: p.public_id,
    name: p.name,
    handler_id: p.handler_id,
  }));

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : isGroup ? [...s, id] : [id]));

  useEffect(() => {
    setSelected([]);
  }, [isGroup]);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex gap-1">
        <ModeTab label="Direct" active={!isGroup} onClick={() => { setIsGroup(false); setSelected([]); }} />
        <ModeTab label="Group" active={isGroup} onClick={() => { setIsGroup(true); setSelected([]); }} />
      </div>
      {isGroup && <Input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Group name" />}
      <PartnerSearchList
        rows={rows}
        search={search}
        onSearchChange={setSearch}
        selected={selected}
        onToggle={toggle}
        multiple={isGroup}
        loading={isLoading}
        emptyLabel="No publishers match."
      />
      <FormActions
        onCancel={onCancel}
        disabled={selected.length === 0 || (isGroup && !title.trim()) || createDirect.isPending || createGroup.isPending}
        onSubmit={() => {
          if (isGroup) {
            createGroup.mutate(
              { title, internal: false, member_ids: selected },
              { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
            );
          } else {
            createDirect.mutate(
              { recipient_account_id: selected[0], context: "general" },
              { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
            );
          }
        }}
      />
    </div>
  );
}

type PlatformKind = "publisher" | "buyer";

function PlatformExternalForm({ onCreated, onCancel }: { onCreated: (id: string) => void; onCancel: () => void }) {
  const [kind, setKind] = useState<PlatformKind>("publisher");
  const [search, setSearch] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [selected, setSelected] = useState("");
  const createDirect = useCreateDirect();

  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => clearTimeout(t);
  }, [search]);

  useEffect(() => {
    setSelected("");
  }, [kind, debouncedSearch]);

  const hasSearch = debouncedSearch.length > 0;
  const filters = { q: debouncedSearch || undefined, page: 1, limit: 25 };
  const publishersQuery = usePlatformPublishers(filters);
  const buyersQuery = usePlatformBuyers(filters);

  const rows =
    kind === "publisher"
      ? hasSearch
        ? (publishersQuery.data?.items ?? [])
        : []
      : hasSearch
        ? (buyersQuery.data?.items ?? [])
        : [];
  const loading = kind === "publisher" ? publishersQuery.isLoading && hasSearch : buyersQuery.isLoading && hasSearch;

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-gray-500">Search for a publisher or buyer to message.</p>
      <div className="flex gap-1">
        <ModeTab label="Publishers" active={kind === "publisher"} onClick={() => setKind("publisher")} />
        <ModeTab label="Buyers" active={kind === "buyer"} onClick={() => setKind("buyer")} />
      </div>
      <Input
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        placeholder="Search name or handler ID…"
      />
      <div className="max-h-56 overflow-y-auto rounded-md border border-gray-100">
        {loading && <p className="p-3 text-center text-xs text-gray-400">Searching…</p>}
        {!loading && !hasSearch && (
          <p className="p-3 text-center text-xs text-gray-400">Type to search {kind === "publisher" ? "publishers" : "buyers"}.</p>
        )}
        {!loading && hasSearch && rows.length === 0 && (
          <p className="p-3 text-center text-xs text-gray-400">No results.</p>
        )}
        {!loading &&
          rows.map((acc) => (
            <label key={acc.id} className="flex cursor-pointer items-center gap-2 px-2 py-1.5 hover:bg-gray-50">
              <input
                type="radio"
                name="platform-account"
                checked={selected === acc.id}
                onChange={() => setSelected(acc.id)}
              />
              <span className="min-w-0 truncate text-sm text-gray-700">{acc.name}</span>
              <span className="ml-auto shrink-0 font-mono text-[11px] text-gray-400">{acc.handler_id}</span>
            </label>
          ))}
      </div>
      <FormActions
        onCancel={onCancel}
        disabled={!selected || createDirect.isPending}
        onSubmit={() =>
          createDirect.mutate(
            { recipient_account_id: selected, context: "general" },
            { onSuccess: (t) => onCreated(t.id), onError: (e) => toast.error(errorMessage(e)) }
          )
        }
      />
    </div>
  );
}

function FormActions({
  onCancel,
  onSubmit,
  disabled,
}: {
  onCancel: () => void;
  onSubmit: () => void;
  disabled: boolean;
}) {
  return (
    <div className="flex justify-end gap-2 pt-1">
      <Button variant="secondary" size="sm" onClick={onCancel}>
        Cancel
      </Button>
      <Button size="sm" onClick={onSubmit} disabled={disabled}>
        Start
      </Button>
    </div>
  );
}
