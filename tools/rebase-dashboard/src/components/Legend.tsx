import { colors } from '../styles/theme'

export function Legend() {
  return (
    <div style={{
      display: 'flex',
      gap: '1.5rem',
      padding: '.5rem 1.5rem',
      borderTop: `1px solid ${colors.border}`,
      fontSize: '.7rem',
      color: colors.muted,
      alignItems: 'center',
    }}>
      <LegendItem color="#3fb950" label="Synced" />
      <LegendItem color="#d29922" label="Deps outdated" />
      <LegendItem color="#58a6ff" label="PR open" />
      <LegendItem color="#8b949e" label="Skipped" />
      <span style={{ color: colors.border }}>|</span>
      <span>Columns = dependency depth</span>
      <span>✓✗ = inline dep status</span>
      <span style={{ marginLeft: 'auto' }}>
        Click card for details
      </span>
      <a href="../" style={{ fontSize: '.7rem' }}>Prow Audit</a>
    </div>
  )
}

function LegendItem({ color, label }: { color: string; label: string }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '.3rem' }}>
      <span style={{
        width: 10, height: 10, borderRadius: 3, display: 'inline-block',
        background: color,
      }} />
      {label}
    </div>
  )
}
