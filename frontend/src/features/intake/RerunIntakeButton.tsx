import { useRerunIntakeQueue } from "@/features/admin/hooks";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import type { QueueItem } from "@/types";

export function RerunIntakeButton({
  item,
  onSuccess,
}: {
  item: QueueItem;
  onSuccess?: (updated: QueueItem) => void;
}) {
  const rerun = useRerunIntakeQueue();

  if (item.status !== "pending_review") return null;

  return (
    <Button
      size="sm"
      disabled={rerun.isPending}
      onClick={() =>
        rerun.mutate(item.id, {
          onSuccess: (updated) => {
            toast.success("Mappings applied");
            onSuccess?.(updated);
          },
          onError: (e) => toast.error(errorMessage(e)),
        })
      }
    >
      Run
    </Button>
  );
}
