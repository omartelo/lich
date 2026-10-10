import { Fragment, type ReactNode } from "react"
import type { At, ParamsOf } from "@/lib/i18n/catalog"
import { interpolate, template, useLocale } from "@/lib/i18n/i18n"
import type { MessageKey, Messages } from "@/lib/i18n/locales/en"

interface TransProps<K extends MessageKey> {
  k: K
  params: ParamsOf<At<Messages, K>, ReactNode>
}

/** A message whose placeholders take markup: a sentence with a link, a code
 * span or a bold name in it stays one key, and each translation places the
 * markup where its own word order puts it. Text-only messages use t(). */
export function Trans<K extends MessageKey>({ k, params }: TransProps<K>) {
  useLocale()
  const values = params as Record<string, ReactNode>
  const count = typeof values.count === "number" ? values.count : undefined
  const parts = interpolate(template(k, count), values)
  return (
    <>
      {parts.map((part, index) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: the parts of one message never reorder between renders
        <Fragment key={index}>{part}</Fragment>
      ))}
    </>
  )
}
