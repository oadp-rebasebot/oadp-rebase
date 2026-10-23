/**
 * Returns a human-readable relative age string for a UTC ISO timestamp, or
 * null when the timestamp is absent or unparseable.
 *
 * Repos are re-scanned independently — a merged CVE fix refreshes only the
 * repo it landed in — so a snapshot can mix scan times and the age tells a
 * viewer which numbers are current.
 */
export function relativeAge(iso: string, now: number = Date.now()): string | null {
  const then = Date.parse(iso)
  if (Number.isNaN(then)) return null

  const minutes = Math.floor((now - then) / 60_000)
  // Clock skew or same-second timestamps should not render "scanned in -2m ago".
  if (minutes < 1) return 'scanned just now'
  if (minutes < 60) return `scanned ${minutes}m ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `scanned ${hours}h ago`

  const days = Math.floor(hours / 24)
  return `scanned ${days}d ago`
}
