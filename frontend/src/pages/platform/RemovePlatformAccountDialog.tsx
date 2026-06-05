import { useEffect, useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/input";

type Props = {
  open: boolean;
  accountName: string;
  accountType: "publisher" | "buyer";
  loading?: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
};

export function RemovePlatformAccountDialog({
  open,
  accountName,
  accountType,
  loading,
  onClose,
  onConfirm,
}: Props) {
  const [confirmText, setConfirmText] = useState("");
  const canRemove = confirmText === "REMOVE" && !loading;
  const label = accountType === "publisher" ? "publisher" : "buyer";

  useEffect(() => {
    if (!open) setConfirmText("");
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={`Remove ${label}?`}
      subtitle={`${accountName} will be removed from platform lists. Data is kept but the account cannot be opened or logged into.`}
      footer={
        <>
          <Button variant="secondary" disabled={loading} onClick={onClose}>
            Cancel
          </Button>
          <Button variant="danger" disabled={!canRemove} onClick={() => void onConfirm()}>
            Remove
          </Button>
        </>
      }
    >
      <div>
        <Label>Type REMOVE to confirm</Label>
        <Input
          value={confirmText}
          onChange={(e) => setConfirmText(e.target.value)}
          placeholder="REMOVE"
          autoComplete="off"
          disabled={loading}
        />
      </div>
    </Dialog>
  );
}
