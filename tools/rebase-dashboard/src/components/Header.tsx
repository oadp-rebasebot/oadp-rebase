import { useState, useEffect } from 'react'
import type { DAGMetadata } from '../types'
import { colors } from '../styles/theme'

interface HeaderProps {
  metadata: DAGMetadata | null
  branch: string
  onBranchChange: (branch: string) => void
}

export function Header({ metadata, branch, onBranchChange }: HeaderProps) {
  const [showHelp, setShowHelp] = useState(false)

  return (
    <>
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: '1rem',
        flexWrap: 'wrap',
        padding: '0.75rem 1.5rem',
        borderBottom: `1px solid ${colors.border}`,
      }}>
        <h1 style={{ fontSize: '1.15rem', fontWeight: 600 }}>
          OADP Rebase Dashboard
        </h1>
        {metadata && (
          <select
            value={branch}
            onChange={e => onBranchChange(e.target.value)}
            style={{
              background: colors.surface,
              border: `1px solid ${colors.border}`,
              color: colors.text,
              padding: '.3rem .5rem',
              borderRadius: 6,
              fontSize: '.85rem',
            }}
          >
            {metadata.branches.map(b => (
              <option key={b} value={b}>{b}</option>
            ))}
          </select>
        )}
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 8 }}>
          <a
            href="https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml"
            target="_blank"
            rel="noopener noreferrer"
            style={{
              display: 'inline-flex', alignItems: 'center',
              background: colors.green + '20',
              border: `1px solid ${colors.green}50`,
              color: colors.green,
              padding: '.3rem .75rem',
              borderRadius: 6,
              fontSize: '.8rem',
              fontWeight: 600,
              textDecoration: 'none',
            }}
          >
            Run rebase for {branch}
          </a>
          <a
            href="https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/rebase-status-wiki.yaml"
            target="_blank"
            rel="noopener noreferrer"
            style={{
              display: 'inline-flex', alignItems: 'center',
              background: colors.blue + '20',
              border: `1px solid ${colors.blue}50`,
              color: colors.blue,
              padding: '.3rem .75rem',
              borderRadius: 6,
              fontSize: '.8rem',
              fontWeight: 600,
              textDecoration: 'none',
            }}
          >
            Refresh status wiki
          </a>
          <button
            onClick={() => setShowHelp(true)}
            style={{
              background: colors.surface2,
              border: `1px solid ${colors.border}`,
              color: colors.muted,
              padding: '.3rem .75rem',
              borderRadius: 6,
              fontSize: '.8rem',
              cursor: 'pointer',
            }}
          >
            ? Help
          </button>
        </div>
      </div>

      {showHelp && <HelpPanel onClose={() => setShowHelp(false)} />}
    </>
  )
}

function HelpPanel({ onClose }: { onClose: () => void }) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <>
      <div onClick={onClose} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', zIndex: 200 }} />
      <div role="dialog" aria-modal="true" aria-label="How OADP Rebases Work" style={{
        position: 'fixed', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
        width: 560, maxHeight: '80vh', overflowY: 'auto',
        background: colors.surface, border: `1px solid ${colors.border}`,
        borderRadius: 12, padding: 24, zIndex: 201,
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <h2 style={{ fontSize: 16, fontWeight: 600, color: colors.text, margin: 0 }}>How OADP Rebases Work</h2>
          <button onClick={onClose} aria-label="Close help" style={{ background: 'none', border: 'none', color: colors.muted, cursor: 'pointer', fontSize: 20 }}>&times;</button>
        </div>

        <div style={{ fontSize: 13, color: colors.text, lineHeight: 1.8 }}>
          <Step n={1} title="Wave ordering">
            Repos rebase in wave order (left to right on this dashboard). Each wave depends on the previous one being merged. Wave 1 repos have no internal OADP dependencies. Wave 2 (velero) depends on wave 1. And so on.
          </Step>

          <Step n={2} title="Check the cards">
            <b style={{ color: colors.green }}>Green</b> = synced, nothing to do.<br/>
            <b style={{ color: colors.yellow }}>Amber</b> = dependencies are outdated, needs rebase.<br/>
            <b style={{ color: colors.blue }}>Blue</b> = rebase PR is open, waiting for review/merge.<br/>
            <b style={{ color: colors.muted }}>Gray</b> = skipped (not actively rebased).
          </Step>

          <Step n={3} title="Dependency lines">
            Hover a card to see its dependencies (lines going left) and dependents (lines going right). The inline <span style={{ color: colors.green }}>✓</span> / <span style={{ color: colors.yellow }}>✗</span> marks show which deps are in sync. Numbers in parentheses show how many commits behind.
          </Step>

          <Step n={4} title="Triggering a rebase">
            Rebases are triggered by running the <b>Auto Rebase</b> GitHub Actions workflow in <a href="https://github.com/oadp-rebasebot/oadp-rebase/actions/workflows/auto-rebase-v2.yaml" target="_blank" rel="noopener noreferrer">oadp-rebase</a>. Select the branches you want to rebase and run the workflow. It will evaluate which repos need rebasing and create PRs automatically.
          </Step>

          <Step n={5} title="After rebase">
            Review the PR created by oadp-rebasebot, comment <code style={{ background: colors.surface2, padding: '2px 6px', borderRadius: 4, fontSize: 12 }}>/ok-to-test</code> to trigger CI, then merge once CI passes. Run the workflow again for the next wave — downstream repos will now detect the updated dependency and become eligible.
          </Step>

          <div style={{ marginTop: 12, padding: '8px 12px', background: colors.surface2, borderRadius: 6, fontSize: 12, color: colors.muted }}>
            Full docs: <a href="https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/docs/auto-rebase.md" target="_blank" rel="noopener noreferrer">auto-rebase.md</a>
          </div>
        </div>
      </div>
    </>
  )
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 14 }}>
      <div style={{ fontSize: 12, fontWeight: 600, color: colors.blue, marginBottom: 2 }}>
        Step {n} — {title}
      </div>
      <div style={{ color: colors.text }}>{children}</div>
    </div>
  )
}
