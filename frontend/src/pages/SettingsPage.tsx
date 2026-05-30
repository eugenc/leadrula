import { useAuthStore } from "@/store/authStore";
import { PageBody } from "@/components/layout/PageBody";
import { Card } from "@/components/ui/misc";
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

export function SettingsPage() {
  const user = useAuthStore((s) => s.user);
  const upload = useUploadMyAvatar();
  return (
    <>
      <PageBody className="max-w-xl">
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
        </Card>
      </PageBody>
    </>
  );
}
