import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { useMe, useUpdateMyAccount } from "@/features/leads/hooks";
import { useAuthStore } from "@/store/authStore";
import { Card } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { Label, Select } from "@/components/ui/input";
import { toast } from "@/store/toastStore";
import { AvatarUpload, uploadError } from "@/features/admin/AvatarUpload";
import { useUploadMyAvatar } from "@/features/admin/hooks";
import { errorMessage } from "@/lib/api";
import { TIMEZONES } from "@/lib/timezones";
import { formatRole } from "@/lib/utils";

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between border-b border-gray-100 py-2 last:border-0">
      <span className="text-sm text-gray-400">{label}</span>
      <span className="text-sm font-medium text-gray-800">{value}</span>
    </div>
  );
}

function HandlerIDRow({
  handlerId,
  accountType,
}: {
  handlerId: string;
  accountType: "buyer" | "publisher" | "platform";
}) {
  const label =
    accountType === "buyer" ? "Buyer ID" : accountType === "platform" ? "Platform ID" : "Publisher ID";
  return (
    <div className="flex items-center justify-between border-b border-gray-100 py-2 last:border-0">
      <div>
        <span className="text-sm text-gray-400">{label}</span>
        <p className="text-xs text-gray-400">Share this so others can link your account</p>
      </div>
      <div className="flex items-center gap-2">
        <code className="text-sm font-semibold text-gray-800">{handlerId}</code>
        <Button
          variant="secondary"
          className="h-7 px-2 text-xs"
          onClick={() => {
            void navigator.clipboard.writeText(handlerId).then(() => toast.success(`Copied ${label}`));
          }}
        >
          Copy
        </Button>
      </div>
    </div>
  );
}

function TimezoneRow({
  timezone,
  savedTimezone,
  isAdmin,
  saving,
  onChange,
  onSave,
}: {
  timezone: string;
  savedTimezone: string;
  isAdmin: boolean;
  saving: boolean;
  onChange: (tz: string) => void;
  onSave: () => void;
}) {
  if (isAdmin) {
    return (
      <div className="flex items-end justify-between gap-3 border-b border-gray-100 py-2">
        <div className="min-w-0 flex-1">
          <Label>Timezone</Label>
          <Select value={timezone} onChange={(e) => onChange(e.target.value)}>
            {TIMEZONES.map((tz) => (
              <option key={tz} value={tz}>
                {tz}
              </option>
            ))}
          </Select>
        </div>
        <Button
          className="shrink-0"
          disabled={timezone === savedTimezone || saving}
          onClick={onSave}
        >
          Save
        </Button>
      </div>
    );
  }

  return <Row label="Timezone" value={timezone || "—"} />;
}

export function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const { data: me } = useMe();
  const upload = useUploadMyAvatar();
  const updateAccount = useUpdateMyAccount();
  const isAdmin = user?.role === "admin";
  const isBuyerAdmin = user?.account_type === "buyer" && isAdmin;
  const handlerId = me?.account.handler_id;
  const accountType = me?.account.type;
  const showTimezone = accountType === "buyer" || accountType === "publisher";
  const savedTimezone = me?.account.timezone ?? "America/Toronto";
  const [timezone, setTimezone] = useState(savedTimezone);

  useEffect(() => {
    setTimezone(savedTimezone);
  }, [savedTimezone]);

  const saveTimezone = () => {
    updateAccount.mutate(timezone, {
      onSuccess: () => toast.success("Timezone updated"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  };

  return (
    <div className="max-w-xl space-y-4">
        <Card className="p-5">
          <div className="mb-5 border-b border-gray-100 pb-5">
            <AvatarUpload
              name={user?.full_name ?? "?"}
              src={user?.avatar_url}
              uploading={upload.isPending}
              onSelect={(file) =>
                upload.mutate(file, {
                  onSuccess: () => toast.success("Photo updated"),
                  onError: uploadError,
                })
              }
            />
          </div>
          <Row label="Name" value={user?.full_name ?? "—"} />
          <Row label="Email" value={user?.email ?? "—"} />
          <Row label="Role" value={user?.role ? formatRole(user.role) : "—"} />
          <Row
            label="Account type"
            value={user?.account_type ? formatRole(user.account_type) : "—"}
          />
          {showTimezone ? (
            <TimezoneRow
              timezone={timezone}
              savedTimezone={savedTimezone}
              isAdmin={isAdmin}
              saving={updateAccount.isPending}
              onChange={setTimezone}
              onSave={saveTimezone}
            />
          ) : null}
          {isAdmin && handlerId && (accountType === "buyer" || accountType === "publisher" || accountType === "platform") ? (
            <HandlerIDRow handlerId={handlerId} accountType={accountType} />
          ) : null}
        </Card>

        {isBuyerAdmin && (
          <Card className="p-5">
            <h2 className="mb-1 text-sm font-semibold text-gray-800">Publisher collaboration</h2>
            <p className="mb-3 text-sm text-gray-500">
              Manage publisher access to this buyer account, approve requests, and view audit history.
            </p>
            <Link
              to="/b/collaboration"
              className="text-sm font-medium text-jade-600 hover:underline"
            >
              Open collaboration settings →
            </Link>
          </Card>
        )}
    </div>
  );
}
