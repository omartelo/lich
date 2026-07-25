import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      // Stock shadcn fills with bg-muted, which in this palette is the same
      // value as the hover fill and all but vanishes against the dark ground —
      // and the pulse spends half its cycle at half opacity on top of that.
      className={cn("animate-pulse rounded-md bg-foreground/20", className)}
      {...props}
    />
  )
}

export { Skeleton }
