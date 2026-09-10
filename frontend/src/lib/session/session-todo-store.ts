import { createKeyedStore, type ReadableKeyedStore } from "@/lib/keyed-store"
import {
  isIdEvent,
  isIdleEvent,
  toSessionTodo,
  type SessionEventSource,
  type SessionTodo,
} from "./session-events"

// Two payloads name the same progress when both counts match. The store builds
// a new object per event, so without this a repeat would hand
// useSyncExternalStore a new reference and re-render the card for nothing.
function sameTodo(a: SessionTodo | null, b: SessionTodo | null): boolean {
  if (a === null || b === null) {
    return a === b
  }
  return a.done === b.done && a.total === b.total
}

// createSessionTodoStore keeps how far each session's agent got through the
// task list it wrote for itself, keyed by session id (see
// terminal.todoEventName). A payload that is not worth drawing clears the
// entry: a list of one, or a finished one, is an answer, not an absence.
//
// It clears on "idle" for the reason the tool line does: SessionEnd means the
// provider's CLI left, and the count would otherwise sit on a card whose agent
// is gone.
export function createSessionTodoStore(
  todoSource: SessionEventSource,
  statusSource: SessionEventSource,
): ReadableKeyedStore<SessionTodo | null> {
  const store = createKeyedStore<SessionTodo | null>(null, sameTodo)

  todoSource((data) => {
    if (isIdEvent(data)) {
      store.set(data.id, toSessionTodo(data))
    }
  })

  statusSource((data) => {
    if (isIdleEvent(data)) {
      store.set(data.id, null)
    }
  })

  return store
}
