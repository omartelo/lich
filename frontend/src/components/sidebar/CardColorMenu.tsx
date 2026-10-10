import { Palette } from "lucide-react"
import type { ComponentType, ReactNode } from "react"
import {
  ContextMenuItem,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from "@/components/ui/context-menu"
import {
  DropdownMenuItem,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu"
import { useT } from "@/lib/i18n/i18n"
import { CARD_COLOR_NAMES, CARD_COLORS, type CardColor } from "@/lib/session/card-color"
import { cn } from "@/lib/utils"

interface CardColorMenuProps {
  // The colour drawn as chosen; undefined is the theme.
  current: CardColor | undefined
  // An empty name hands the card back to the theme.
  onPick: (color: string) => void
}

type SwatchItem = ComponentType<{
  className?: string
  "aria-label": string
  title: string
  onClick: () => void
  children: ReactNode
}>

function Swatches({ current, onPick, Item }: CardColorMenuProps & { Item: SwatchItem }) {
  const t = useT()
  const options: (CardColor | "")[] = ["", ...CARD_COLOR_NAMES]
  return (
    <div className="grid grid-cols-3 gap-0.5">
      {options.map((name) => {
        const label = t(`sidebar.cardColor.color.${name || "theme"}`)
        const on = (current ?? "") === name
        return (
          <Item
            key={name || "theme"}
            aria-label={on ? t("sidebar.cardColor.current", { color: label }) : label}
            title={label}
            onClick={() => onPick(name)}
            className="justify-center p-1.5"
          >
            <span
              className={cn(
                "size-4 rounded-full ring-1 ring-foreground/15",
                on && "outline-2 outline-offset-2 outline-foreground",
                !name && "bg-[conic-gradient(var(--accent)_0_50%,var(--background)_0)]",
              )}
              style={name ? { background: CARD_COLORS[name] } : undefined}
            />
          </Item>
        )
      })}
    </div>
  )
}

// "Color" on a session card's right-click menu.
export function CardColorContextSub(props: CardColorMenuProps) {
  const t = useT()
  return (
    <ContextMenuSub>
      <ContextMenuSubTrigger>
        <Palette />
        {t("sidebar.cardColor.menu")}
      </ContextMenuSubTrigger>
      <ContextMenuSubContent className="min-w-0 shadow-lg">
        <Swatches {...props} Item={ContextMenuItem} />
      </ContextMenuSubContent>
    </ContextMenuSub>
  )
}

// "Color" on a folder header's options menu: it paints every card in the folder.
export function CardColorDropdownSub(props: CardColorMenuProps) {
  const t = useT()
  return (
    <DropdownMenuSub>
      <DropdownMenuSubTrigger>
        <Palette />
        {t("sidebar.cardColor.menu")}
      </DropdownMenuSubTrigger>
      <DropdownMenuSubContent className="min-w-0">
        <Swatches {...props} Item={DropdownMenuItem} />
      </DropdownMenuSubContent>
    </DropdownMenuSub>
  )
}
