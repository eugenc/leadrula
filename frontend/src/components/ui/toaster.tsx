import { cn } from "@/lib/utils";
import { useToastStore } from "@/store/toastStore";
import { X } from "lucide-react";

export function Toaster() {
  const { toasts, dismiss } = useToastStore();
  return (
    <div className="fixed bottom-4 left-4 z-[100] flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cn(
            "flex min-w-[240px] items-start gap-2 rounded-md px-4 py-2.5 text-sm font-medium text-white shadow-lg",
            t.variant === "success" && "bg-jade-500",
            t.variant === "error" && "bg-danger",
            t.variant === "default" && "bg-gray-800"
          )}
        >
          <span className="flex-1">{t.message}</span>
          <button
            type="button"
            aria-label="Dismiss"
            onClick={() => dismiss(t.id)}
            className="shrink-0 rounded p-0.5 opacity-80 hover:opacity-100"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
    </div>
  );
}
