// Sets a field's value the way a keystroke would. React tracks the value an
// input last held through the instance's own setter, so a plain `field.value =`
// is a change it never sees; the prototype's setter, then an input event, is
// what fires its onChange.
export function typeInto(field: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  Object.getOwnPropertyDescriptor(Object.getPrototypeOf(field), "value")?.set?.call(field, value)
  field.dispatchEvent(new Event("input", { bubbles: true }))
}
