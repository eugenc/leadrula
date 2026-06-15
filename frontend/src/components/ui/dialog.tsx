import { IconButton } from "@/components/layout/IconButton";
import { cn } from "@/lib/utils";
import { X } from "lucide-react";

export const drawerTitleClass = "text-base font-semibold text-gray-800";
export const drawerSubtitleClass = "mt-0.5 text-xs text-gray-400";
export const formFieldClass =
  "[&_label]:!mb-1 [&_label]:!text-sm [&_input]:!h-8 [&_input]:!text-sm [&_select]:!h-8 [&_select]:!text-sm [&_textarea]:!min-h-[60px] [&_textarea]:!text-sm";

export function Dialog({
  open,
  onClose,
  title,
  subtitle,
  children,
  className,
  footer,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  subtitle?: string;
  children: React.ReactNode;
  className?: string;
  footer?: React.ReactNode;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex animate-fadeIn items-center justify-center">
      <div
        className="absolute inset-0 bg-[var(--surface-overlay)]"
        onClick={onClose}
      />
      <div
        className={cn(
          "relative z-10 flex w-full max-w-[400px] flex-col gap-4 rounded-lg bg-surface-card p-5 shadow-lg",
          className
        )}
      >
        {(title || subtitle) && (
          <div>
            {title && <h3 className={drawerTitleClass}>{title}</h3>}
            {subtitle && <p className={drawerSubtitleClass}>{subtitle}</p>}
          </div>
        )}
        <div className={formFieldClass}>{children}</div>
        {footer && <div className="flex justify-end gap-2 pt-1">{footer}</div>}
      </div>
    </div>
  );
}

export function DialogHeader({
  title,
  onClose,
}: {
  title: string;
  onClose: () => void;
}) {
  return (
    <div className="flex items-center justify-between border-b border-gray-100 px-6 py-4">
      <h3 className="text-lg font-semibold text-gray-800">{title}</h3>
      <button
        onClick={onClose}
        className="text-gray-400 hover:text-gray-700"
        aria-label="Close"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}

export function Sheet({
  open,
  onClose,
  children,
  width = 480,
}: {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
  width?: number;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-[60]">
      <div
        className="absolute inset-0 animate-fadeIn bg-[var(--surface-overlay)]"
        onClick={onClose}
      />
      <div
        className="absolute right-0 top-0 flex h-full animate-slideInRight flex-col overflow-y-auto bg-surface-card shadow-xl"
        style={{ width }}
      >
        {children}
      </div>
    </div>
  );
}

export const drawerHeaderClass =
  "flex items-start justify-between border-b border-gray-100 px-5 py-3.5";
export const drawerBodyClass = cn("flex-1 overflow-y-auto px-5 py-4", formFieldClass);
export const drawerFooterClass = "border-t border-gray-100 px-5 py-3";

export function DrawerHeader({
  title,
  subtitle,
  detail,
  onClose,
}: {
  title: string;
  subtitle?: string;
  detail?: string;
  onClose: () => void;
}) {
  return (
    <div className={drawerHeaderClass}>
      <div>
        <div className={drawerTitleClass}>{title}</div>
        {subtitle && <div className={drawerSubtitleClass}>{subtitle}</div>}
        {detail && <div className="mt-0.5 font-mono text-xs text-gray-400">{detail}</div>}
      </div>
      <IconButton onClick={onClose} aria-label="Close">
        <X className="h-4 w-4" />
      </IconButton>
    </div>
  );
}

export function DrawerBody({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn(drawerBodyClass, className)}>{children}</div>;
}

export function DrawerFooter({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return <div className={cn(drawerFooterClass, className)}>{children}</div>;
}

export function FormDrawer({
  open,
  onClose,
  title,
  subtitle,
  children,
  footer,
  width = 480,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  width?: number;
}) {
  return (
    <Sheet open={open} onClose={onClose} width={width}>
      <div className="flex h-full flex-col">
        <DrawerHeader title={title} subtitle={subtitle} onClose={onClose} />
        <DrawerBody>{children}</DrawerBody>
        {footer && <DrawerFooter className="flex justify-end gap-2">{footer}</DrawerFooter>}
      </div>
    </Sheet>
  );
}
