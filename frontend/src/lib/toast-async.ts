import {toast} from "sonner"
import {errorText} from "./utils"

// runWithToast runs an async action behind a single toast: a loading spinner
// that resolves in place to a success message or, on a thrown error,
// "<failure>: <message>". Returns whether the action succeeded. Shared by the
// startup update gates (the app and the Claude plugin), which otherwise repeat
// this loading→success/error dance verbatim.
export async function runWithToast(
  loading: string,
  action: () => Promise<unknown>,
  success: string,
  failure: string,
): Promise<boolean> {
  const id = toast.loading(loading)
  try {
    await action()
    toast.success(success, {id})
    return true
  } catch (error) {
    toast.error(`${failure}: ${errorText(error)}`, {id})
    return false
  }
}
