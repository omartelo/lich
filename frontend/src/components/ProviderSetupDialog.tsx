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

interface ProviderSetupDialogProps {
  /** Whether any harness was found on PATH. The dialog has two shapes. */
  hasInstalled: boolean
  /** Display names of every harness lich knows, for the nothing-installed shape. */
  knownNames: string[]
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
export function ProviderSetupDialog({
  hasInstalled,
  knownNames,
  onDone,
}: ProviderSetupDialogProps) {
  return (
    <Dialog open onOpenChange={() => {}}>
      <DialogContent showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>
            {hasInstalled ? "Choose your agents" : "No coding agents found"}
          </DialogTitle>
          <DialogDescription>
            {hasInstalled
              ? "lich found these on your machine. Turn on the ones you use — the rest stay out of the New Session menu."
              : "lich runs the agents already installed on your machine, and found none. Install one and lich picks it up on the next launch."}
          </DialogDescription>
        </DialogHeader>
        {/* Nothing installed means nothing to pick, so the panel would be five
            dead switches under a default nobody can honour. The list of names is
            the whole answer that shape owes. */}
        {hasInstalled ? (
          <ProvidersSettings installedOnly />
        ) : (
          <DialogDescription>lich looks for {knownNames.join(", ")}.</DialogDescription>
        )}
        <DialogDescription>
          {hasInstalled
            ? "You can change this any time in Settings › Providers."
            : "Already have one installed? Point lich at its binary in Settings › Providers."}
        </DialogDescription>
        <DialogFooter>
          <Button onClick={onDone}>Continue</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
