import { onAppEvent } from "@/lib/app-events"
import { STATUS_EVENT, TODO_EVENT, type SessionTodo } from "./session-events"
import { createSessionTodoStore } from "./session-todo-store"
import { useKeyedStore } from "@/lib/use-keyed-store"

// Subscribed at import rather than on first use: that opens the /events socket
// at page load, so progress reported before any card mounts still lands.
const store = createSessionTodoStore(
  (handler) => onAppEvent(TODO_EVENT, handler),
  (handler) => onAppEvent(STATUS_EVENT, handler),
)

// useSessionTodo reads how far a session's agent got through the task list it
// wrote for itself (see session-todo-store), which retains it across the card's
// unmount. Null whenever there is no list worth drawing, which is most
// sessions: the agent wrote none, its provider writes none lich can read, or
// the one it wrote is finished.
export function useSessionTodo(sessionId: string): SessionTodo | null {
  return useKeyedStore(store, sessionId)
}
