import { useEffect, useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

type Props = {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle: string;
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
};

export function DeletePipelineResourceConfirmDialog({
  open,
  onClose,
  title,
  subtitle,
  loading,
  onConfirm,
}: Props) {
  const [confirmText, setConfirmText] = useState("");
  const canDelete = confirmText === "DELETE" && !loading;

  useEffect(() => {
    if (!open) setConfirmText("");
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      subtitle={subtitle}
      footer={
        <>
          <Button variant="secondary" disabled={loading} onClick={onClose}>
            Cancel
          </Button>
          <Button variant="danger" disabled={!canDelete} onClick={() => void onConfirm()}>
            Delete
          </Button>
        </>
      }
    >
      <div>
        <Label>Type DELETE to confirm</Label>
        <Input
          value={confirmText}
          onChange={(e) => setConfirmText(e.target.value)}
          placeholder="DELETE"
          autoComplete="off"
          disabled={loading}
          className="mt-1.5"
        />
      </div>
    </Dialog>
  );
}
