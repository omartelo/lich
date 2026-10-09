// The types that keep every locale in step with English. en is written
// `as const`, so each message keeps its literal text and the placeholders in
// it become the parameters t() demands. Every other locale is checked against
// Shape<en>: a key missing or extra there is a tsc error, not a blank string at
// runtime. That the placeholders match across locales is a runtime check
// (i18n.test.ts), since a translation is free to reorder them.

/** A message that changes with a number. `other` is required because every
 * CLDR rule falls back to it; the rest are the forms Intl.PluralRules can
 * select, written only where the language uses them. */
export interface PluralForms {
  zero?: string
  one?: string
  two?: string
  few?: string
  many?: string
  other: string
}

type IsPlural<T> = T extends { readonly other: string } ? true : false

/** The shape a translation must fill: the same tree as English, with every
 * text widened to any string. */
export type Shape<T> = {
  [K in keyof T]: T[K] extends string
    ? string
    : IsPlural<T[K]> extends true
      ? PluralForms
      : Shape<T[K]>
}

/** Every dotted path from the root to a message. */
export type Paths<T> = {
  [K in keyof T & string]: T[K] extends string
    ? K
    : IsPlural<T[K]> extends true
      ? K
      : `${K}.${Paths<T[K]>}`
}[keyof T & string]

/** The message a dotted path points at. */
export type At<T, P extends string> = P extends `${infer Head}.${infer Rest}`
  ? Head extends keyof T
    ? At<T[Head], Rest>
    : never
  : P extends keyof T
    ? T[P]
    : never

/** The `{name}` placeholders in a template. */
export type Placeholders<S> = S extends `${string}{${infer Name}}${infer Rest}`
  ? Name | Placeholders<Rest>
  : never

type Texts<M> = M extends string ? M : M[keyof M]

/** What a message needs filled in: its placeholders, and `count` as a number
 * for a plural. */
export type ParamsOf<M, V> =
  IsPlural<M> extends true
    ? { count: number } & Record<Exclude<Placeholders<Texts<M>>, "count">, V>
    : Record<Placeholders<M>, V>

/** t()'s trailing argument: absent for a message with no placeholders. */
export type ParamsArg<M, V> = [Placeholders<Texts<M>>] extends [never]
  ? IsPlural<M> extends true
    ? [params: { count: number }]
    : []
  : [params: ParamsOf<M, V>]
