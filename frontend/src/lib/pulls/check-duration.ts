// How long a check took, from gh's ISO timestamps. A finished check reports the
// span it ran; one still going reports how long it has been going, which is the
// number a waiting reviewer actually wants. Pure so the node gate covers it.
//
// now is injectable for the same reason: a running check's duration depends on
// the clock, and a test should not.
export function checkDuration(
  startedAt: string,
  completedAt: string,
  now: number = Date.now(),
): string {
  const start = Date.parse(startedAt)
  if (Number.isNaN(start)) {
    return ""
  }
  const end = completedAt ? Date.parse(completedAt) : now
  if (Number.isNaN(end) || end < start) {
    return ""
  }
  return formatSeconds(Math.round((end - start) / 1000))
}

// Rounded to the unit that reads at a glance: seconds under a minute, minutes
// and seconds under an hour, hours and minutes past it.
function formatSeconds(total: number): string {
  if (total < 60) {
    return `${total}s`
  }
  const minutes = Math.floor(total / 60)
  if (minutes < 60) {
    const seconds = total % 60
    return seconds === 0 ? `${minutes}m` : `${minutes}m ${seconds}s`
  }
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  return rest === 0 ? `${hours}h` : `${hours}h ${rest}m`
}
