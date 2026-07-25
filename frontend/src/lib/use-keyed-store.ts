import { useCallback, useSyncExternalStore } from "react"
import type { ReadableKeyedStore } from "./keyed-store"

// useKeyedStore reads one key out of a shared module-level store (see
// keyed-store) and re-renders when that key alone changes. The store keeps the
// value across the component's unmount, which is the whole point: a session
// card lives only while its project is on screen.
//
// An empty key subscribes to nothing — a component with no session or no path
// yet must not open an entry, and for the git-status store it must not start a
// poll on "". The snapshot still answers the store's empty value.
export function useKeyedStore<T>(store: ReadableKeyedStore<T>, key: string): T {
  const subscribe = useCallback(
    (onChange: () => void) => (key ? store.subscribe(key, onChange) : () => {}),
    [store, key],
  )
  return useSyncExternalStore(subscribe, () => store.get(key))
}
