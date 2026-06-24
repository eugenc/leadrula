import { useEffect, useState } from "react";
import { useAuthStore } from "@/store/authStore";
import { Button } from "@/components/ui/button";
import { Input, Label, FilterSelect } from "@/components/ui/input";
import { Card } from "@/components/ui/misc";
import { Settings2, Trash2 } from "lucide-react";
import {
  type DashboardView,
  type WidgetId,
  type DashboardGoals,
  type DashboardPeriod,
  type DashboardPeriodState,
  widgetCatalog,
  defaultDashboardView,
  useDashboardViews,
  useCreateDashboardView,
  useUpdateDashboardView,
  useDeleteDashboardView,
  useActiveDashboardViewId,
  useDashboardPeriod,
  resolveDashboardView,
  PERIOD_OPTIONS,
  periodBounds,
} from "./dashboardViews";

function emptyDraft(isBuyer: boolean): DashboardView {
  const base = defaultDashboardView(isBuyer);
  return { ...base, public_id: "", name: "New view", widgets: [...base.widgets] };
}

export function DashboardViewPicker({
  view,
  onViewChange,
}: {
  view: DashboardView;
  onViewChange: (v: DashboardView) => void;
}) {
  const user = useAuthStore((s) => s.user);
  const isBuyer = user?.account_type === "buyer";
  const isAdmin = user?.role === "admin";
  const catalog = widgetCatalog(!!isBuyer);

  const { data: views } = useDashboardViews();
  const { activeId, setActiveId } = useActiveDashboardViewId();
  const { periodState, setPeriod } = useDashboardPeriod();
  const createView = useCreateDashboardView();
  const updateView = useUpdateDashboardView();
  const deleteView = useDeleteDashboardView();

  const savedViews = views ?? [];
  const hasSaved = savedViews.length > 0;

  const [configureOpen, setConfigureOpen] = useState(false);
  const [draft, setDraft] = useState<DashboardView>(() => view);
  const [isNew, setIsNew] = useState(false);

  useEffect(() => {
    if (!configureOpen) {
      setDraft(view);
      setIsNew(false);
    }
  }, [view, configureOpen]);

  const selectView = async (publicId: string) => {
    if (publicId === "default") {
      onViewChange(defaultDashboardView(!!isBuyer));
      if (hasSaved) await setActiveId("");
      return;
    }
    const v = savedViews.find((s) => s.public_id === publicId);
    if (v) {
      onViewChange(v);
      await setActiveId(publicId);
    }
  };

  const toggleWidget = (id: WidgetId) => {
    setDraft((d) => ({
      ...d,
      widgets: d.widgets.includes(id) ? d.widgets.filter((w) => w !== id) : [...d.widgets, id],
    }));
  };

  const moveWidget = (id: WidgetId, dir: -1 | 1) => {
    setDraft((d) => {
      const idx = d.widgets.indexOf(id);
      if (idx < 0) return d;
      const next = [...d.widgets];
      const swap = idx + dir;
      if (swap < 0 || swap >= next.length) return d;
      [next[idx], next[swap]] = [next[swap]!, next[idx]!];
      return { ...d, widgets: next };
    });
  };

  const setGoals = (patch: Partial<DashboardGoals>) => {
    setDraft((d) => ({ ...d, goals: { ...d.goals, ...patch } }));
  };

  const resetDraft = () => {
    setDraft((d) => ({
      ...d,
      widgets: [],
      goals: {},
    }));
  };

  const saveDraft = async () => {
    const body = {
      name: draft.name.trim(),
      widgets: draft.widgets,
      period: "all",
      goals: draft.goals,
    };
    if (!body.name) return;

    if (isNew || !draft.public_id || draft.public_id === "default") {
      const created = await createView.mutateAsync(body);
      onViewChange(created);
      await setActiveId(created.public_id);
    } else {
      const updated = await updateView.mutateAsync({
        id: draft.public_id,
        body,
      });
      onViewChange(updated);
      await setActiveId(updated.public_id);
    }
    setConfigureOpen(false);
  };

  const removeView = async () => {
    if (!draft.public_id || draft.public_id === "default") return;
    await deleteView.mutateAsync(draft.public_id);
    const fallback = resolveDashboardView(
      savedViews.filter((v) => v.public_id !== draft.public_id),
      "",
      !!isBuyer
    );
    onViewChange(fallback);
    if (fallback.public_id !== "default") {
      await setActiveId(fallback.public_id);
    } else {
      await setActiveId("");
    }
    setConfigureOpen(false);
  };

  const editCurrent = () => {
    if (view.public_id === "default") {
      setDraft(emptyDraft(!!isBuyer));
      setIsNew(true);
    } else {
      setDraft({ ...view, widgets: [...view.widgets] });
      setIsNew(false);
    }
    setConfigureOpen(true);
  };

  const selectValue = view.public_id === "default" && !activeId ? "default" : view.public_id;

  const onPeriodChange = (period: DashboardPeriod) => {
    const next: DashboardPeriodState = { period };
    if (period === "custom") {
      const { start, end } = periodBounds({ period: "7days" });
      next.customFrom = periodState.customFrom ?? start.toISOString().slice(0, 10);
      next.customTo = periodState.customTo ?? end.toISOString().slice(0, 10);
    }
    void setPeriod(next);
  };

  const onCustomDateChange = (field: "customFrom" | "customTo", value: string) => {
    void setPeriod({
      period: "custom",
      customFrom: field === "customFrom" ? value : periodState.customFrom,
      customTo: field === "customTo" ? value : periodState.customTo,
    });
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-2">
          <FilterSelect
            value={selectValue}
            onChange={(e) => selectView(e.target.value)}
            className="w-auto min-w-[180px]"
          >
            {!hasSaved && <option value="default">Default</option>}
            {savedViews.map((v) => (
              <option key={v.public_id} value={v.public_id}>
                {v.name}
              </option>
            ))}
          </FilterSelect>

          {isAdmin && (
            <>
              <Button variant="secondary" size="sm" onClick={editCurrent}>
                <Settings2 className="h-4 w-4" />
                Configure
              </Button>
            </>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <FilterSelect
            value={periodState.period}
            onChange={(e) => onPeriodChange(e.target.value as DashboardPeriod)}
            className="w-auto min-w-[140px]"
            aria-label="Period"
          >
            {PERIOD_OPTIONS.map((p) => (
              <option key={p.value} value={p.value}>
                {p.label}
              </option>
            ))}
          </FilterSelect>
          {periodState.period === "custom" && (
            <>
              <Input
                type="date"
                value={periodState.customFrom ?? ""}
                onChange={(e) => onCustomDateChange("customFrom", e.target.value)}
                className="h-8 w-auto px-3 text-sm text-gray-700"
                aria-label="From date"
              />
              <Input
                type="date"
                value={periodState.customTo ?? ""}
                onChange={(e) => onCustomDateChange("customTo", e.target.value)}
                className="h-8 w-auto px-3 text-sm text-gray-700"
                aria-label="To date"
              />
            </>
          )}
        </div>
      </div>

      {configureOpen && isAdmin && (
        <Card className="p-4">
          <div className="mb-4">
            <Label>View name</Label>
            <Input
              id="view-name"
              value={draft.name}
              onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))}
            />
          </div>

          <div className="mb-4 grid gap-3 sm:grid-cols-2">
            <div>
              <Label>Lead target (goals widget)</Label>
              <Input
                id="lead-target"
                type="number"
                min={0}
                value={draft.goals.lead_target ?? ""}
                onChange={(e) =>
                  setGoals({
                    lead_target: e.target.value === "" ? undefined : Number(e.target.value),
                  })
                }
              />
            </div>
            <div>
              <Label>Revenue target (goals widget)</Label>
              <Input
                id="revenue-target"
                type="number"
                min={0}
                step={0.01}
                value={draft.goals.revenue_target ?? ""}
                onChange={(e) =>
                  setGoals({
                    revenue_target: e.target.value === "" ? undefined : Number(e.target.value),
                  })
                }
              />
            </div>
          </div>

          <div className="mb-4">
            <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
              Widgets
            </div>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {catalog.map((w) => {
                const checked = draft.widgets.includes(w.id);
                const idx = draft.widgets.indexOf(w.id);
                return (
                  <div
                    key={w.id}
                    className="flex items-center gap-2 rounded-md border border-gray-100 px-2 py-1.5"
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggleWidget(w.id)}
                      className="rounded border-gray-300"
                    />
                    <span className="flex-1 text-sm text-gray-700">{w.label}</span>
                    {checked && (
                      <span className="flex gap-0.5">
                        <button
                          type="button"
                          className="px-1 text-xs text-gray-400 hover:text-gray-700"
                          onClick={() => moveWidget(w.id, -1)}
                          disabled={idx === 0}
                        >
                          ↑
                        </button>
                        <button
                          type="button"
                          className="px-1 text-xs text-gray-400 hover:text-gray-700"
                          onClick={() => moveWidget(w.id, 1)}
                          disabled={idx === draft.widgets.length - 1}
                        >
                          ↓
                        </button>
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              onClick={saveDraft}
              disabled={createView.isPending || updateView.isPending || !draft.name.trim()}
            >
              {isNew || view.public_id === "default" ? "Save view" : "Update view"}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setConfigureOpen(false)}>
              Cancel
            </Button>
            <Button variant="ghost" size="sm" onClick={resetDraft}>
              Reset
            </Button>
            {!isNew && draft.public_id && draft.public_id !== "default" && (
              <Button
                variant="danger"
                size="sm"
                onClick={removeView}
                disabled={deleteView.isPending}
              >
                <Trash2 className="h-4 w-4" />
                Delete
              </Button>
            )}
          </div>
        </Card>
      )}
    </div>
  );
}

export function useResolvedDashboardView(isBuyer: boolean) {
  const { data: views, isLoading: viewsLoading } = useDashboardViews();
  const { activeId, isLoading: prefsLoading } = useActiveDashboardViewId();
  const [view, setView] = useState<DashboardView>(() => defaultDashboardView(isBuyer));

  useEffect(() => {
    if (viewsLoading || prefsLoading) return;
    setView(resolveDashboardView(views, activeId, isBuyer));
  }, [views, activeId, isBuyer, viewsLoading, prefsLoading]);

  return { view, setView, viewsLoading: viewsLoading || prefsLoading };
}
