import { MessageSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { errorMessage } from "@/lib/api";
import { toast } from "@/store/toastStore";
import { useChatStore } from "@/store/chatStore";
import { useOpenLeadThread, useOpenContractThread } from "./hooks";

// MessageButton opens the chat widget to a (context-aware) thread with an account.
export function MessageButton({
  accountId,
  leadId,
  contractId,
  context = "general",
  label = "Message",
  variant = "secondary",
  size = "sm",
  className,
  iconOnly = false,
}: {
  accountId: string;
  leadId?: string;
  contractId?: string;
  context?: "general" | "lead" | "contract";
  label?: string;
  variant?: "primary" | "secondary" | "ghost" | "outline";
  size?: "sm" | "md" | "lg" | "icon";
  className?: string;
  iconOnly?: boolean;
}) {
  const openWithRecipient = useChatStore((s) => s.openWithRecipient);
  return (
    <Button
      variant={variant}
      size={iconOnly ? "icon" : size}
      className={cn(className)}
      onClick={(e) => {
        e.stopPropagation();
        openWithRecipient(accountId, { context, leadId, contractId });
      }}
    >
      <MessageSquare className="h-4 w-4" />
      {!iconOnly && label}
    </Button>
  );
}

// LeadMessageButton opens a lead-context thread with the lead's counterpart.
export function LeadMessageButton({
  leadId,
  label = "Message",
  variant = "secondary",
  size = "sm",
  className,
  iconOnly = false,
}: {
  leadId: string;
  label?: string;
  variant?: "primary" | "secondary" | "ghost" | "outline";
  size?: "sm" | "md" | "lg" | "icon";
  className?: string;
  iconOnly?: boolean;
}) {
  const openThread = useChatStore((s) => s.openThread);
  const open = useOpenLeadThread();
  return (
    <Button
      variant={variant}
      size={iconOnly ? "icon" : size}
      className={cn(className)}
      disabled={open.isPending}
      onClick={(e) => {
        e.stopPropagation();
        open.mutate(leadId, { onSuccess: (t) => openThread(t.id), onError: (err) => toast.error(errorMessage(err)) });
      }}
    >
      <MessageSquare className="h-4 w-4" />
      {!iconOnly && label}
    </Button>
  );
}

// ContractMessageButton opens a contract-context thread with the counterpart.
export function ContractMessageButton({
  contractId,
  label = "Message",
  variant = "secondary",
  size = "sm",
  className,
  iconOnly = false,
}: {
  contractId: string;
  label?: string;
  variant?: "primary" | "secondary" | "ghost" | "outline";
  size?: "sm" | "md" | "lg" | "icon";
  className?: string;
  iconOnly?: boolean;
}) {
  const openThread = useChatStore((s) => s.openThread);
  const open = useOpenContractThread();
  return (
    <Button
      variant={variant}
      size={iconOnly ? "icon" : size}
      className={cn(className)}
      disabled={open.isPending}
      onClick={(e) => {
        e.stopPropagation();
        open.mutate(contractId, { onSuccess: (t) => openThread(t.id), onError: (err) => toast.error(errorMessage(err)) });
      }}
    >
      <MessageSquare className="h-4 w-4" />
      {!iconOnly && label}
    </Button>
  );
}
