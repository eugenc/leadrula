import type { AccountType, EffectivePermissionsPayload, Role, UserRow } from "@/types";
import {
  ACTION_OPTIONS,
  type EffectivePermissions,
  type LeadScope,
  LeadScopeAll,
  LeadScopeAssigned,
  leadScopeFromFlags,
  leadScopeToFlags,
  navSections,
  presetForRole,
  resolvePermissions,
} from "@/lib/permissions";

interface Props {
  role: Role;
  accountType: AccountType;
  effective: EffectivePermissions;
  onChange: (effective: EffectivePermissions) => void;
}

export function PermissionsEditor({ role, accountType, effective, onChange }: Props) {
  const sections = navSections(accountType);
  const { assigned, followed } = leadScopeToFlags(effective.lead_scope);
  const seesAllLeads = effective.lead_scope === LeadScopeAll;

  function setNav(key: string, value: boolean) {
    onChange({ ...effective, nav: { ...effective.nav, [key]: value } });
  }

  function setAction(key: keyof EffectivePermissions["actions"], value: boolean) {
    onChange({ ...effective, actions: { ...effective.actions, [key]: value } });
  }

  function setLeadVisibility(nextAssigned: boolean, nextFollowed: boolean) {
    if (!nextAssigned && !nextFollowed) {
      if (role === "admin") {
        onChange({ ...effective, lead_scope: LeadScopeAll });
      }
      return;
    }
    const scope = leadScopeFromFlags(nextAssigned, nextFollowed, role);
    if (scope) onChange({ ...effective, lead_scope: scope });
  }

  function resetToPreset() {
    onChange(presetForRole(role, accountType));
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
          Lead visibility
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={assigned}
              onChange={(e) => setLeadVisibility(e.target.checked, followed)}
            />
            Assigned to me
          </label>
          <label className="flex items-center gap-2 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={followed}
              onChange={(e) => setLeadVisibility(assigned, e.target.checked)}
            />
            Followed by me
          </label>
        </div>
        {seesAllLeads && (
          <p className="mt-2 text-xs text-gray-500">Sees all leads (Admin role default)</p>
        )}
      </div>

      {sections.map((section) => (
        <div key={section.label}>
          <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
            {section.label}
          </div>
          <div className="flex flex-col gap-1.5">
            {section.keys.map(({ key, label }) => (
              <label key={key} className="flex items-center gap-2 text-sm text-gray-700">
                <input
                  type="checkbox"
                  checked={!!effective.nav[key]}
                  onChange={(e) => setNav(key, e.target.checked)}
                />
                {label}
              </label>
            ))}
          </div>
        </div>
      ))}

      <div>
        <div className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-400">
          Actions
        </div>
        <div className="flex flex-col gap-1.5">
          {ACTION_OPTIONS.map(({ key, label }) => (
            <label key={key} className="flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={!!effective.actions[key]}
                onChange={(e) => setAction(key, e.target.checked)}
              />
              {label}
            </label>
          ))}
        </div>
      </div>

      <button
        type="button"
        className="text-left text-sm text-jade-700 hover:underline"
        onClick={resetToPreset}
      >
        Reset to role defaults
      </button>
    </div>
  );
}

interface UserRowLike {
  permissions?: Record<string, unknown>;
  effective_permissions?: EffectivePermissionsPayload;
}

export function effectiveFromUserRow(
  role: Role,
  accountType: AccountType,
  row?: UserRowLike | UserRow
): EffectivePermissions {
  if (row?.effective_permissions) {
    const ep = row.effective_permissions;
    const actions = ep.actions ?? {};
    return {
      nav: ep.nav ?? {},
      lead_scope: (ep.lead_scope as LeadScope) ?? LeadScopeAssigned,
      actions: {
        settings_admin: !!actions.settings_admin,
        pipelines_routing: !!actions.pipelines_routing,
        contracts_partners: !!actions.contracts_partners,
        billing: !!actions.billing,
        appointments_manage: !!actions.appointments_manage,
      },
    };
  }
  return resolvePermissions(role, accountType, row?.permissions);
}

export function emptyOverrides(role: Role, accountType: AccountType): Record<string, unknown> {
  return resolvePermissions(role, accountType, {}) as unknown as Record<string, unknown>;
}
