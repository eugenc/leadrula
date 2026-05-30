import { cn } from "@/lib/utils";
import { useToastStore } from "@/store/toastStore";

export function Toaster() {
  const { toasts, dismiss } = useToastStore();
  return (
    <div className="fixed bottom-4 left-4 z-[100] flex flex-col gap-2">
      {toasts.map((t) => (
        <div
          key={t.id}
          onClick={() => dismiss(t.id)}
          className={cn(
            "min-w-[240px] cursor-pointer rounded-md px-4 py-2.5 text-sm font-medium text-white shadow-lg",
            t.variant === "success" && "bg-jade-500",
            t.variant === "error" && "bg-danger",
            t.variant === "default" && "bg-gray-800"
          )}
        >
          {t.message}
        </div>
      ))}
    </div>
  );
}
