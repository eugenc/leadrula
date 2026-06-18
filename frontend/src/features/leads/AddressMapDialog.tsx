import { useEffect, useState } from "react";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";
import { fetchGoogleMapsSatelliteMap } from "@/features/integrations/hooks";
import { errorMessage } from "@/lib/api";

function mapDialogError(err: unknown): string {
  const msg = errorMessage(err);
  const lower = msg.toLowerCase();
  if (lower.includes("maps static api") || lower.includes("not authorized")) {
    return "Enable Maps Static API on your Google Cloud key (Integrations → Google Maps).";
  }
  return msg.replace(/^google static map failed:\s*/i, "");
}

export function AddressMapDialog({
  open,
  onClose,
  placeId,
  formattedAddress,
}: {
  open: boolean;
  onClose: () => void;
  placeId: string;
  formattedAddress: string;
}) {
  const [zoom, setZoom] = useState(18);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setError(null);
      setZoom(18);
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
        setImageUrl(null);
      }
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchGoogleMapsSatelliteMap(placeId, zoom)
      .then((blob) => {
        if (cancelled) return;
        const url = URL.createObjectURL(blob);
        setImageUrl((prev) => {
          if (prev) URL.revokeObjectURL(prev);
          return url;
        });
      })
      .catch((err) => {
        if (!cancelled) setError(mapDialogError(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [open, placeId, zoom]);

  useEffect(() => {
    return () => {
      if (imageUrl) URL.revokeObjectURL(imageUrl);
    };
  }, [imageUrl]);

  if (!open) return null;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Property location"
      subtitle={formattedAddress}
      className="max-w-[640px]"
      footer={
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className="relative h-[400px] overflow-hidden rounded-md border border-gray-100">
        {loading && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-surface-card/80">
            <Spinner className="h-6 w-6" />
          </div>
        )}
        {error ? (
          <div className="flex h-full items-center justify-center px-4 text-sm text-gray-500">{error}</div>
        ) : imageUrl ? (
          <img src={imageUrl} alt={formattedAddress} className="h-full w-full object-cover" />
        ) : null}
        {!error && (
          <div className="absolute left-3 top-3 z-10 flex flex-col gap-1">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="h-7 min-w-7 px-2"
              disabled={loading || zoom >= 20}
              onClick={() => setZoom((z) => Math.min(20, z + 1))}
            >
              +
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="h-7 min-w-7 px-2"
              disabled={loading || zoom <= 15}
              onClick={() => setZoom((z) => Math.max(15, z - 1))}
            >
              −
            </Button>
          </div>
        )}
      </div>
    </Dialog>
  );
}
