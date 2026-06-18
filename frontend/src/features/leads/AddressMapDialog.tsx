import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { Dialog } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/misc";
import { fetchGoogleMapsPlaceDetails } from "@/features/integrations/hooks";
import { errorMessage } from "@/lib/api";

const ESRI_SATELLITE =
  "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}";

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
      setError(null);
      if (mapInstance.current) {
        mapInstance.current.remove();
        mapInstance.current = null;
      }
      return;
    }

    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchGoogleMapsPlaceDetails(placeId)
      .then((details) => {
        if (cancelled || !mapRef.current) return;
        if (details.lat === 0 && details.lng === 0) {
          setError("Location not found for this address");
          return;
        }

        if (mapInstance.current) {
          mapInstance.current.remove();
          mapInstance.current = null;
        }

        const map = L.map(mapRef.current, {
          center: [details.lat, details.lng],
          zoom: 18,
        });

        L.tileLayer(ESRI_SATELLITE, {
          attribution: "Tiles &copy; Esri",
          maxZoom: 19,
        }).addTo(map);

        L.circleMarker([details.lat, details.lng], {
          radius: 8,
          color: "#fff",
          weight: 2,
          fillColor: "#dc2626",
          fillOpacity: 1,
        }).addTo(map);

        mapInstance.current = map;
        requestAnimationFrame(() => map.invalidateSize());
      })
      .catch((err) => {
        if (!cancelled) setError(errorMessage(err));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
      if (mapInstance.current) {
        mapInstance.current.remove();
        mapInstance.current = null;
      }
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
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-surface-card/80">
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
