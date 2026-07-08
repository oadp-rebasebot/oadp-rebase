import { useEffect, useRef } from 'react'
import confetti from 'canvas-confetti'
import type { RepoData } from '../types'
import { classifyStatus } from '../dag-data'
import { colors } from '../styles/theme'

interface SummaryBarProps {
  repos: RepoData[]
  branch: string
  loading: boolean
  generatedAt?: string
}

export function SummaryBar({ repos, branch, loading, generatedAt }: SummaryBarProps) {
  const confettiFired = useRef(new Set<string>())

  const active = repos.filter(r => !r.skip)
  const counts = { done: 0, prog: 0, need: 0 }
  for (const r of active) {
    const s = classifyStatus(r)
    if (s === 'done') counts.done++
    else if (s === 'prog') counts.prog++
    else if (s === 'need') counts.need++
  }
  const total = active.length || 1
  const pct = Math.round((counts.done / total) * 100)
  const allDone = active.length > 0 && counts.done === active.length

  useEffect(() => {
    if (allDone && !loading && branch && !confettiFired.current.has(branch)) {
      confettiFired.current.add(branch)
      setTimeout(() => {
        confetti({ particleCount: 60, spread: 55, origin: { x: 0.3, y: 0.6 }, colors: ['#3fb950', '#58a6ff', '#d29922', '#ffffff'] })
        setTimeout(() => confetti({ particleCount: 60, spread: 55, origin: { x: 0.7, y: 0.6 }, colors: ['#3fb950', '#58a6ff', '#d29922', '#ffffff'] }), 250)
        setTimeout(() => confetti({ particleCount: 40, spread: 70, origin: { x: 0.5, y: 0.5 }, colors: ['#3fb950', '#58a6ff', '#d29922', '#ffffff'] }), 500)
      }, 400)
    }
  }, [allDone, loading, branch])

  const timestamp = generatedAt
    ? new Date(generatedAt).toLocaleString('en-US', {
        month: 'short', day: 'numeric',
        hour: '2-digit', minute: '2-digit', hour12: true,
        timeZone: 'America/New_York',
      }) + ' ET'
    : ''

  return (
    <div style={{
      display: 'flex',
      gap: '1.5rem',
      padding: '.4rem 1.5rem',
      fontSize: '.8rem',
      alignItems: 'center',
      borderBottom: `1px solid ${colors.border}`,
      background: colors.surface,
    }}>
      <Stat num={counts.done} label="synced" color={colors.green} />
      <Stat num={counts.prog} label="in progress" color={colors.blue} />
      <Stat num={counts.need} label="outdated" color={colors.yellow} />

      <div style={{
        flex: 1, maxWidth: 200, height: 6,
        background: colors.surface2, borderRadius: 3, overflow: 'hidden',
      }}>
        <div style={{
          height: '100%', background: colors.green, borderRadius: 3,
          width: `${pct}%`, transition: 'width 0.4s ease',
        }} />
      </div>

      {allDone && <span style={{ fontSize: '1rem' }}>🎉</span>}

      {timestamp && (
        <span style={{ marginLeft: 'auto', fontSize: '.7rem', color: colors.muted }}>
          Updated {timestamp}
        </span>
      )}
    </div>
  )
}

function Stat({ num, label, color }: { num: number; label: string; color: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '.35rem' }}>
      <span style={{ fontWeight: 700, fontSize: '1rem', color }}>{num}</span>
      <span style={{ color: colors.muted, fontSize: '.7rem', textTransform: 'uppercase', letterSpacing: '.04em' }}>{label}</span>
    </div>
  )
}
