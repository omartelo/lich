import type {ComponentType} from "react"
import {File} from "lucide-react"
// Deep per-icon imports, NOT the devicons-react barrel: the barrel is one CJS
// module re-exporting every icon, so importing from it bundles all ~4000 of
// them (~10MB). The package's exports map allows ./icons/*, so each icon is its
// own ~1.4KB module.
import BashOriginal from "devicons-react/icons/BashOriginal"
import Css3Original from "devicons-react/icons/Css3Original"
import GoOriginal from "devicons-react/icons/GoOriginal"
import Html5Original from "devicons-react/icons/Html5Original"
import JavascriptOriginal from "devicons-react/icons/JavascriptOriginal"
import JsonOriginal from "devicons-react/icons/JsonOriginal"
import KotlinOriginal from "devicons-react/icons/KotlinOriginal"
import MarkdownOriginal from "devicons-react/icons/MarkdownOriginal"
import PythonOriginal from "devicons-react/icons/PythonOriginal"
import RustOriginal from "devicons-react/icons/RustOriginal"
import TypescriptOriginal from "devicons-react/icons/TypescriptOriginal"
import XmlOriginal from "devicons-react/icons/XmlOriginal"
import YamlOriginal from "devicons-react/icons/YamlOriginal"
import {extname} from "./lang-badge"

// devicons-react components spread extra props onto their <svg> and take a
// `size` that sets both dimensions. Only the props this module passes are
// typed — a narrower shape than the icons' own SVGProps, which they satisfy.
type IconComponent = ComponentType<{
  size?: number | string
  fill?: string
  className?: string
}>

// ICONS maps a file extension to its devicon. `mono` flags the marks that ship
// with no fill of their own (they render black and vanish on a dark ground), so
// those ride the row's text color via fill=currentColor; every other icon keeps
// its brand colors. Extensions absent here fall back to a neutral file glyph.
const ICONS: Record<string, {Icon: IconComponent; mono?: boolean}> = {
  ts: {Icon: TypescriptOriginal},
  tsx: {Icon: TypescriptOriginal},
  js: {Icon: JavascriptOriginal},
  jsx: {Icon: JavascriptOriginal},
  go: {Icon: GoOriginal},
  css: {Icon: Css3Original},
  html: {Icon: Html5Original},
  json: {Icon: JsonOriginal},
  kt: {Icon: KotlinOriginal},
  kts: {Icon: KotlinOriginal},
  md: {Icon: MarkdownOriginal, mono: true},
  xml: {Icon: XmlOriginal},
  yaml: {Icon: YamlOriginal},
  yml: {Icon: YamlOriginal},
  sh: {Icon: BashOriginal},
  py: {Icon: PythonOriginal},
  rs: {Icon: RustOriginal, mono: true},
}

// ICON_PX matches the tree's size-3.5 (14px) folder icons so rows line up.
const ICON_PX = 14

interface FileIconProps {
  path: string
}

export function FileIcon({path}: FileIconProps) {
  const def = ICONS[extname(path)]
  if (!def) {
    return <File className="size-3.5 shrink-0 text-muted-foreground opacity-70"/>
  }
  const {Icon, mono} = def
  return (
    <Icon
      size={ICON_PX}
      fill={mono ? "currentColor" : undefined}
      className="shrink-0"
    />
  )
}
