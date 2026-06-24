import { useEffect, useState } from "react";
import { getBlob } from "@/lib/api";
import { Spinner } from "@/components/ui/misc";

// Recordings are served via an auth-gated proxy, so we fetch the blob with the
// Bearer token and play it from an object URL rather than a bare <audio src>.
export function RecordingPlayer({ role, callId }: { role: "publisher" | "buyer"; callId: number }) {
  const [url, setUrl] = useState<string | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    let revoked: string | null = null;
    let active = true;
    setUrl(null);
    setError(false);
    getBlob(`/${role}/calls/${callId}/recording`)
      .then((blob) => {
        if (!active) return;
        const objUrl = URL.createObjectURL(blob);
        revoked = objUrl;
        setUrl(objUrl);
      })
      .catch(() => active && setError(true));
    return () => {
      active = false;
      if (revoked) URL.revokeObjectURL(revoked);
    };
  }, [role, callId]);

  if (error) return <p className="mt-2 text-sm text-gray-400">Recording unavailable.</p>;
  if (!url)
    return (
      <div className="mt-2 flex items-center gap-2 text-sm text-gray-400">
        <Spinner /> Loading recording…
      </div>
    );
  return <audio controls className="mt-2 w-full" src={url} />;
}
