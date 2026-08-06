import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { formatMoney } from "@/lib/utils";

type Props = {
  open: boolean;
  buyerName: string;
  amount: number;
  note?: string;
  loading?: boolean;
  onClose: () => void;
  onConfirm: () => void | Promise<void>;
};

export function CreditPlatformBuyerDialog({
  open,
  buyerName,
  amount,
  note,
  loading,
  onClose,
  onConfirm,
}: Props) {
  const trimmedNote = note?.trim();

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Add funds?"
      subtitle={`Add ${formatMoney(amount)} to ${buyerName}.`}
      footer={
        <>
          <Button variant="secondary" disabled={loading} onClick={onClose}>
            Cancel
          </Button>
          <Button disabled={loading} onClick={() => void onConfirm()}>
            Confirm
          </Button>
        </>
      }
    >
      {trimmedNote ? (
        <p className="text-sm text-gray-600">
          Note: <span className="text-gray-800">{trimmedNote}</span>
        </p>
      ) : null}
    </Dialog>
  );
}
