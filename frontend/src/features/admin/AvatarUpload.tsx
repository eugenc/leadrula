import { useRef } from "react";
import { Avatar, Spinner } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { apiError } from "@/lib/api";

const ACCEPT = "image/jpeg,image/png,image/webp";

export function AvatarUpload({
  name,
  src,
  uploading,
  onSelect,
}: {
  name: string;
  src?: string | null;
  uploading: boolean;
  onSelect: (file: File) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);

  function pick(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (file.size > 2 << 20) {
      toast.error("Image must be 2 MB or smaller");
      return;
    }
    onSelect(file);
  }

  return (
    <div className="flex items-center gap-4">
      <Avatar name={name} src={src} className="h-14 w-14 text-lg" />
      <div>
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPT}
          onChange={pick}
          className="hidden"
        />
        <Button
          variant="secondary"
          size="sm"
          disabled={uploading}
          onClick={() => inputRef.current?.click()}
        >
          {uploading ? <Spinner className="text-gray-500" /> : "Change photo"}
        </Button>
        <p className="mt-1 text-xs text-gray-400">JPEG, PNG, or WebP. Max 2 MB.</p>
      </div>
    </div>
  );
}

export function uploadError(e: unknown) {
  toast.error(apiError(e).message);
}
