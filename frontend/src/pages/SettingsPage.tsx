import { Link } from "react-router-dom";
import { useMe } from "@/features/leads/hooks";
import { useAuthStore } from "@/store/authStore";
import { PageBody } from "@/components/layout/PageBody";
import { Card } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { AvatarUpload, uploadError } from "@/features/admin/AvatarUpload";
import { useUploadMyAvatar } from "@/features/admin/hooks";
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

export function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const { data: me } = useMe();
  const upload = useUploadMyAvatar();
  const isAdmin = user?.role === "admin";
  const isBuyerAdmin = user?.account_type === "buyer" && isAdmin;
  const handlerId = me?.account.handler_id;
  const accountType = me?.account.type;

  return (
    <>
      <PageBody className="max-w-xl space-y-4">
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
      </PageBody>
    </>
  );
}
