import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";
import { fetchGoogleMapsPlaceDetails } from "@/features/integrations/hooks";
import { errorMessage } from "@/lib/api";

const pinIcon = L.divIcon({
  className: "",
  html: `<div style="width:24px;height:24px;margin-left:-12px;margin-top:-24px;background:#dc2626;border:2px solid #fff;border-radius:50% 50% 50% 0;transform:rotate(-45deg);box-shadow:0 2px 6px rgba(0,0,0,.3)"></div>`,
  iconSize: [24, 24],
  iconAnchor: [12, 24],
});

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
  const mapRef = useRef<HTMLDivElement>(null);
  const mapInstance = useRef<L.Map | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      mapInstance.current?.remove();
      mapInstance.current = null;
      setError(null);
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchGoogleMapsPlaceDetails(placeId)
      .then((details) => {
        if (cancelled || !mapRef.current) return;
        if (!details.lat && !details.lng) {
          setError("Location not found for this address");
          return;
        }
        mapInstance.current?.remove();
        const map = L.map(mapRef.current, { zoomControl: true }).setView([details.lat, details.lng], 16);
        L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
          attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
          maxZoom: 19,
        }).addTo(map);
        L.marker([details.lat, details.lng], { icon: pinIcon }).addTo(map);
        mapInstance.current = map;
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      mapInstance.current?.remove();
      mapInstance.current = null;
    };
  }, [open, placeId]);

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
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-white/80">
            <Spinner className="h-6 w-6" />
          </div>
        )}
        {error ? (
          <div className="flex h-full items-center justify-center px-4 text-sm text-gray-500">{error}</div>
        ) : (
          <div ref={mapRef} className="h-full w-full" />
        )}
      </div>
    </Dialog>
  );
}
