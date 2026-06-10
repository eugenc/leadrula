import { useEffect, useState } from "react";
import { useAuthStore } from "@/store/authStore";
import { Card, Switch } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import { toast } from "@/store/toastStore";
import { errorMessage } from "@/lib/api";
import {
  useNotificationSettings,
  useUpdateNotificationSettings,
} from "@/hooks/useNotifications";
import {
  accountNotificationEvents,
  LEAD_NOTIFICATION_EVENTS,
  withDefaults,
} from "@/features/notifications/settings";
import type { NotificationPrefs } from "@/types";

function PrefsTable({
  title,
  description,
  events,
  prefs,
  onChange,
}: {
  title: string;
  description: string;
  events: { id: string; label: string }[];
  prefs: NotificationPrefs;
  onChange: (eventId: string, channel: "in_app" | "email", value: boolean) => void;
}) {
  return (
    <Card className="p-5">
      <h2 className="mb-1 text-sm font-semibold text-gray-800">{title}</h2>
      <p className="mb-4 text-sm text-gray-500">{description}</p>
      <div className="overflow-hidden rounded-lg border border-gray-100">
        <div className="grid grid-cols-[1fr_5rem_5rem] gap-2 border-b border-gray-100 bg-gray-50 px-4 py-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
          <span>Event</span>
          <span className="text-center">In-app</span>
          <span className="text-center">Email</span>
        </div>
        {events.map((e) => {
          const row = prefs[e.id];
          return (
            <div
              key={e.id}
              className="grid grid-cols-[1fr_5rem_5rem] items-center gap-2 border-b border-gray-100 px-4 py-3 last:border-0"
            >
              <span className="text-sm font-medium text-gray-800">{e.label}</span>
              <div className="flex justify-center">
                <Switch
                  checked={row?.in_app ?? true}
                  onChange={(v) => onChange(e.id, "in_app", v)}
                />
              </div>
              <div className="flex justify-center">
                <Switch
                  checked={row?.email ?? false}
                  onChange={(v) => onChange(e.id, "email", v)}
                />
              </div>
            </div>
          );
        })}
      </div>
    </Card>
  );
}

export function NotificationsSettingsPage() {
  const user = useAuthStore((s) => s.user);
  const isAdmin = user?.role === "admin";
  const accountType = user?.account_type;
  const showAccount = isAdmin && accountType !== "platform";
  const showPersonal = !isAdmin;

  const { data, isLoading } = useNotificationSettings();
  const update = useUpdateNotificationSettings();

  const accountEvents =
    accountType && showAccount ? accountNotificationEvents(accountType) : [];
  const [accountPrefs, setAccountPrefs] = useState<NotificationPrefs>({});
  const [personalPrefs, setPersonalPrefs] = useState<NotificationPrefs>({});

  useEffect(() => {
    if (!data) return;
    if (data.account) {
      setAccountPrefs(withDefaults(data.account, accountEvents));
    }
    setPersonalPrefs(withDefaults(data.personal, LEAD_NOTIFICATION_EVENTS));
  }, [data, accountEvents.length]);

  if (isLoading || !data) {
    return <p className="text-sm text-gray-400">Loading notification settings…</p>;
  }

  const patchChannel = (
    setter: React.Dispatch<React.SetStateAction<NotificationPrefs>>,
    eventId: string,
    channel: "in_app" | "email",
    value: boolean,
  ) => {
    setter((prev) => ({
      ...prev,
      [eventId]: {
        in_app: channel === "in_app" ? value : (prev[eventId]?.in_app ?? true),
        email: channel === "email" ? value : (prev[eventId]?.email ?? false),
      },
    }));
  };

  const save = () => {
    const body: { account?: NotificationPrefs; personal?: NotificationPrefs } = {};
    if (showAccount) body.account = accountPrefs;
    if (showPersonal) body.personal = personalPrefs;
    update.mutate(body, {
      onSuccess: () => toast.success("Notification settings saved"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  };

  if (!showAccount && !showPersonal) {
    return (
      <p className="text-sm text-gray-500">Notification settings are not available for this account.</p>
    );
  }

  return (
    <div className="max-w-2xl space-y-4">
      {showAccount ? (
        <PrefsTable
          title="Account notifications"
          description="Applies to all admins on this account."
          events={accountEvents}
          prefs={accountPrefs}
          onChange={(id, ch, v) => patchChannel(setAccountPrefs, id, ch, v)}
        />
      ) : null}
      {showPersonal ? (
        <PrefsTable
          title="Lead notifications"
          description="Sent when you are the assignee on a lead."
          events={LEAD_NOTIFICATION_EVENTS}
          prefs={personalPrefs}
          onChange={(id, ch, v) => patchChannel(setPersonalPrefs, id, ch, v)}
        />
      ) : null}
      <Button onClick={save} disabled={update.isPending}>
        Save
      </Button>
    </div>
  );
}
