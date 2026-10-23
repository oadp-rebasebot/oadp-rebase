import { describe, expect, it } from 'vitest'
import { relativeAge } from './relativeAge'

// A fixed "now" so the assertions do not depend on the wall clock.
const now = Date.parse('2026-01-02T12:00:00Z')

function minutesAgo(minutes: number): string {
  return new Date(now - minutes * 60_000).toISOString()
}

describe('relativeAge', () => {
  it('reports very recent scans as just now', () => {
    expect(relativeAge(minutesAgo(0), now)).toBe('scanned just now')
  })

  it('collapses sub-minute ages into just now', () => {
    expect(relativeAge(new Date(now - 20_000).toISOString(), now)).toBe('scanned just now')
  })

  it('reports minutes', () => {
    expect(relativeAge(minutesAgo(12), now)).toBe('scanned 12m ago')
  })

  it('reports hours past the hour mark', () => {
    expect(relativeAge(minutesAgo(150), now)).toBe('scanned 2h ago')
  })

  it('reports days past the day mark', () => {
    expect(relativeAge(minutesAgo(60 * 24 * 3), now)).toBe('scanned 3d ago')
  })

  it('does not render a negative age when the scan clock is ahead', () => {
    expect(relativeAge(minutesAgo(-10), now)).toBe('scanned just now')
  })

  it('returns null for an unparseable timestamp so the label is omitted', () => {
    expect(relativeAge('not-a-date', now)).toBeNull()
  })
})
