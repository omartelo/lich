import { ExternalLink } from "lucide-react"
import { ProviderIcon } from "@/components/ProviderIcon"
import { CheckAgainButton } from "@/components/common/CheckAgainButton"
import { ProvidersSettings } from "@/components/settings/ProvidersSettings"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { refreshProviders, type ProviderState } from "@/lib/providers-store"
import { System } from "@/lib/rpc"
import { useT } from "@/lib/i18n/i18n"

interface ProviderSetupDialogProps {
  /** Whether any harness was found on PATH. The dialog has two shapes. */
  hasInstalled: boolean
  /** Every harness lich knows, for the nothing-installed shape: each row opens
   * the page documenting how to install that one. */
  known: ProviderState[]
  /** Confirm — the gate records the default, which is what closes it for good. */
  onDone: () => void
}

// The screen a new lich opens on: which coding agents to offer, and which one new
// sessions spawn. It is the Settings › Providers panel verbatim, so the first
// screen a user meets is the one they find again when they want to change it.
//
// Continue is the only way out, so the dialog can never leave without writing the
// choice it exists to collect: no close button, and `open` is pinned true while
// onOpenChange swallows Escape and the backdrop (base-ui has no dismissible flag
// on Dialog — a controlled root that refuses to close is the equivalent).
export function ProviderSetupDialog({ hasInstalled, known, onDone }: ProviderSetupDialogProps) {
  const t = useT()
  return (
    <Dialog open onOpenChange={() => {}}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>
            {hasInstalled
              ? t("shell.providerSetupDialog.chooseTitle")
              : t("shell.providerSetupDialog.noneTitle")}
          </DialogTitle>
          <DialogDescription>
            {hasInstalled
              ? t("shell.providerSetupDialog.chooseDescription")
              : t("shell.providerSetupDialog.noneDescription")}
          </DialogDescription>
        </DialogHeader>
        {/* Nothing installed means nothing to pick, so the panel would be seven
            dead switches under a default nobody can honour. The names are the
            whole answer that shape owes — each one linking to its own install
            page, which is what makes the screen a way out rather than a wall. */}
        {hasInstalled ? (
          <ProvidersSettings />
        ) : (
          <div className="grid grid-cols-2 gap-x-4">
            {known.map((provider) => (
              <button
                key={provider.id}
                type="button"
                className="group -mx-2 flex items-center gap-2 rounded-md px-2 py-1.5 text-sm text-foreground hover:bg-accent/60"
                onClick={() => void System.OpenExternal(provider.docs)}
              >
                <ProviderIcon kind={provider.id} />
                {provider.name}
                <ExternalLink className="ml-auto size-3 text-muted-foreground opacity-0 group-hover:opacity-100" />
              </button>
            ))}
          </div>
        )}
        <DialogDescription>
          {hasInstalled
            ? t("shell.providerSetupDialog.changeLater")
            : t("shell.providerSetupDialog.pointToBinary")}
        </DialogDescription>
        <DialogFooter>
          {!hasInstalled && <CheckAgainButton onCheck={refreshProviders} size="default" />}
          <Button onClick={onDone}>{t("shell.providerSetupDialog.continue")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
