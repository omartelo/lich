import ReactMarkdown, { type Components } from "react-markdown"
import remarkGfm from "remark-gfm"
import { System } from "@/lib/rpc"
import { cn } from "@/lib/utils"

// mdProse styles react-markdown's output through lich tokens with descendant
// selectors — one place, no per-element component map. Covers the GitHub dialect
// a PR body is written in: headings, lists, fenced code, tables, quotes, images.
const mdProse = cn(
  "text-sm leading-relaxed text-muted-foreground break-words",
  "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
  "[&_p]:my-2",
  "[&_h1]:mt-4 [&_h1]:mb-2 [&_h1]:text-lg [&_h1]:font-semibold [&_h1]:text-foreground",
  "[&_h2]:mt-4 [&_h2]:mb-2 [&_h2]:text-base [&_h2]:font-semibold [&_h2]:text-foreground",
  "[&_h3]:mt-2.5 [&_h3]:mb-1 [&_h3]:font-semibold [&_h3]:text-foreground",
  "[&_h4]:mt-2 [&_h4]:font-semibold [&_h4]:text-foreground",
  "[&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-0.5",
  "[&_code]:rounded [&_code]:bg-accent [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[0.85em]",
  "[&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-md [&_pre]:bg-muted [&_pre]:p-2.5",
  "[&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-[0.8125rem]",
  "[&_blockquote]:border-l-2 [&_blockquote]:border-border [&_blockquote]:pl-3 [&_blockquote]:italic",
  "[&_hr]:my-3 [&_hr]:border-border",
  "[&_a]:font-medium [&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-2",
  "[&_strong]:font-semibold [&_strong]:text-foreground",
  "[&_img]:my-2 [&_img]:max-w-full [&_img]:rounded",
  "[&_table]:my-2 [&_table]:block [&_table]:w-max [&_table]:max-w-full [&_table]:overflow-x-auto",
  "[&_th]:border [&_th]:border-border [&_th]:px-2 [&_th]:py-1 [&_th]:text-left [&_th]:font-medium [&_th]:text-foreground",
  "[&_td]:border [&_td]:border-border [&_td]:px-2 [&_td]:py-1",
  "[&_input]:mr-1 [&_input]:align-middle",
)

// A markdown link must not navigate lich's own Chromium window; open it in the
// system browser instead, the same as every other external link in the app. A
// PR body also carries anchors and repo-relative links, which the backend's
// http(s) gate refuses — swallow those here instead of firing a rejected call.
const components: Components = {
  a(props) {
    const { href, children } = props
    return (
      <a
        href={href}
        onClick={(e) => {
          e.preventDefault()
          if (href?.startsWith("http://") || href?.startsWith("https://")) {
            void System.OpenExternal(href)
          }
        }}
      >
        {children}
      </a>
    )
  },
}

// Markdown renders untrusted GitHub markdown (a PR body) as a styled React tree.
// react-markdown ignores raw HTML by default — no sanitizer needed — and
// remark-gfm adds tables, task lists, strikethrough and autolinks.
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div className={cn(mdProse, className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </ReactMarkdown>
    </div>
  )
}
