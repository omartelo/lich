import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Trans } from "@/components/common/Trans"
import { Button } from "@/components/ui/button"
import { useT } from "@/lib/i18n/i18n"
import { clearStageMove, useStageMove } from "@/lib/session/stage-move-store"
import type { Panes } from "@/lib/session/use-panes"

// Adding a session that is already on another wall takes it off that one —
// somebody else's arrangement, changed by a click aimed at this one. So the
// decision goes to the user rather than being made under them; `add` refuses
// the move until they have answered. Mounted once beside the stage: the sidebar
// is not always (it collapses to the rail), and the palette and a pane's + ask
// the same question.
export function StageMoveDialog({ panes }: { panes: Panes }) {
  const t = useT()
  const moving = useStageMove()
  const confirm = () => {
    if (moving) {
      panes.add(moving.session.id, { move: true })
    }
    clearStageMove()
  }
  return (
    <ConfirmDialog
      open={!!moving}
      onCancel={clearStageMove}
      // "this split" only when there is one: adding to no wall starts a new one
      // around the active session, and naming a split the user cannot see is the
      // same lie as the entry that promised to stop showing a card.
      title={
        panes.current
          ? t("sidebar.stageMoveDialog.title", { name: moving?.session.label ?? "" })
          : t("sidebar.stageMoveDialog.showBesideTitle", { name: moving?.session.label ?? "" })
      }
      description={
        moving?.from.cells.length === 2 ? (
          <Trans
            k="sidebar.stageMoveDialog.endsGroup"
            params={{ group: <strong>{moving.from.name}</strong> }}
          />
        ) : (
          <Trans
            k="sidebar.stageMoveDialog.leavesSplit"
            params={{ group: <strong>{moving?.from.name}</strong> }}
          />
        )
      }
    >
      <Button onClick={confirm}>{t("sidebar.stageMoveDialog.confirm")}</Button>
    </ConfirmDialog>
  )
}
