# Session management improvements: file change inventory

This is the approved implementation inventory for the per-project default harness and checkout-group session menu.

| File | Change |
| --- | --- |
| `frontend/src/lib/providers-store.ts` | Add project-scoped default-provider loading, persistence, subscriptions, and resolution while preserving the global fallback. |
| `frontend/src/lib/providers-store.test.ts` | Cover project/global precedence, loading, disabled overrides, persistence, and clearing. |
| `frontend/src/providers/projects.tsx` | Resolve implicit UI-created sessions against the target project's default and preload project settings before a project becomes interactive. |
| `frontend/src/components/settings/SessionSettings.tsx` | Add the Project Settings > Sessions pane with the harness selector and `Use default` action. |
| `frontend/src/components/settings/Settings.tsx` | Group navigation into Global, Project, and Provider settings and register the Sessions pane. |
| `frontend/src/components/settings/ProvidersSettings.tsx` | Clarify that the global default is the fallback for projects without an override. |
| `frontend/src/components/sidebar/SessionSidebar.tsx` | Pass the enabled-provider roster into checkout groups. |
| `frontend/src/components/sidebar/SessionGroup.tsx` | Add the checkout-header `+` menu for enabled harnesses and a new terminal, without adding it to Pinned or lone headerless groups. |
| `CHANGELOG.md` | Document both user-visible improvements under `[Unreleased]`. |

No SQLite schema, Go service, RPC method, or API type change is planned: the existing `settings` table and `Store.GetSetting`/`Store.SetSetting` RPC methods already support project-scoped values.

The project override uses the existing `provider.default` key with a non-empty project scope. An empty project value inherits the unchanged global `provider.default` value.
