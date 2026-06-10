import { useEffect, useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

type Props = {
  open: boolean;
  onClose: () => void;
  contractName: string;
  buyerLabel?: string;
  loading?: boolean;
  onConfirm: () => void | Promise<void>;
};

export function DeleteContractConfirmDialog({
  open,
  onClose,
  contractName,
  buyerLabel,
  loading,
  onConfirm,
}: Props) {
  const [confirmText, setConfirmText] = useState("");
  const canDelete = confirmText === "DELETE" && !loading;

  useEffect(() => {
    if (!open) setConfirmText("");
  }, [open]);

  const subtitle = buyerLabel
    ? `Permanently delete "${contractName}" (${buyerLabel})?`
    : `Permanently delete "${contractName}"?`;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Delete contract?"
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
