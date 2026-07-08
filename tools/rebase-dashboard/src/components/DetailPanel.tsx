import { useEffect, useState } from 'react'
import type { RepoData } from '../types'
import { colors } from '../styles/theme'

interface DetailPanelProps {
  repo: RepoData
  onClose: () => void
}

const statusIcon: Record<string, string> = {
  ok: '✅',
  fail: '❌',
  warn: '⚠️',
  skip: '⏭️',
  na: '➖',
}

const checkMeta: Record<string, { label: string; tip?: string }> = {
  open_pr: { label: 'Rebase PR', tip: 'Open rebase PR from oadp-rebasebot' },
  config: { label: 'Rebase Config', tip: 'Rebase config file exists in oadp-rebase repo' },
  rebasebot: { label: 'Rebasebot Branch', tip: 'Scratch branch exists in oadp-rebasebot fork' },
  go_version: { label: 'Go Version', tip: 'Go version from go.mod (toolchain if set)' },
  ci_config: { label: 'Prow CI Config', tip: 'ci-operator config exists in openshift/release' },
  dep_sync: { label: 'Dep Sync', tip: 'Internal OADP dependencies are at the latest commit' },
  gomod_drift: { label: 'Go.mod Drift', tip: 'go.mod dependency versions match upstream' },
  konflux: { label: 'Konflux', tip: 'Konflux build config and Dockerfile present' },
  upstream_image: { label: 'Quay Image', tip: 'Container image exists on quay.io' },
  art_config: { label: 'ART Config', tip: 'ocp-build-data build config exists' },
  productized: { label: 'Productized', tip: 'Entry in bundle/image-references' },
}

export function DetailPanel({ repo, onClose }: DetailPanelProps) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  const prNum = repo.checks?.open_pr?.summary?.replace('#', '')

  return (
    <>
      <div
        onClick={onClose}
        style={{
          position: 'absolute', inset: 0,
          background: 'rgba(0,0,0,0.3)',
          zIndex: 100,
        }}
      />
      <div style={{
        position: 'absolute', top: 0, right: 0, bottom: 0,
        width: 380,
        background: colors.surface,
        borderLeft: `1px solid ${colors.border}`,
        zIndex: 101,
        overflowY: 'auto',
        padding: 20,
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <div>
            <h2 style={{ fontSize: 16, fontWeight: 600, color: colors.text, margin: 0 }}>
              {repo.repo}
            </h2>
            <div style={{ fontSize: 12, color: colors.muted, marginTop: 2 }}>
              Wave {repo.wave} &middot; {repo.branch}
            </div>
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'none', border: 'none', color: colors.muted,
              cursor: 'pointer', fontSize: 18, padding: 4,
            }}
          >
            &times;
          </button>
        </div>

        {repo.upstream && (() => {
          const [orgRepo, ref] = repo.upstream.split(' @ ')
          const upstreamUrl = `https://github.com/${orgRepo}${ref ? `/tree/${ref}` : ''}`
          return (
            <div style={{ fontSize: 12, color: colors.muted }}>
              Upstream: <a href={upstreamUrl} target="_blank" rel="noopener noreferrer" style={{ color: colors.text }}>{repo.upstream}</a>
            </div>
          )
        })()}

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          <a
            href={`https://github.com/${repo.repo}/tree/${repo.branch}`}
            target="_blank"
            rel="noopener noreferrer"
            style={linkButtonStyle}
          >
            Repo
          </a>
          {prNum && (
            <a
              href={`https://github.com/${repo.repo}/pull/${prNum}`}
              target="_blank"
              rel="noopener noreferrer"
              style={linkButtonStyle}
            >
              PR #{prNum}
            </a>
          )}
          <a
            href={`https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/rebase-configs/${repo.repo.replace('/', '_').replace(/-/g, '_')}_${repo.branch}.env.sh`}
            target="_blank"
            rel="noopener noreferrer"
            style={linkButtonStyle}
          >
            Rebase Config
          </a>
          <a
            href={`https://github.com/${repo.repo}/blob/${repo.branch}/go.mod`}
            target="_blank"
            rel="noopener noreferrer"
            style={linkButtonStyle}
          >
            go.mod
          </a>
        </div>

        <Section title="Checks">
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 12 }}>
            <tbody>
              {Object.entries(repo.checks ?? {})
                .filter(([, check]) => check.status !== 'na' && check.status !== 'skip')
                .map(([id, check]) => (
                <tr key={id} style={{ borderBottom: `1px solid ${colors.border}`, position: 'relative' }}>
                  <td style={{ padding: '4px 0', color: colors.muted, cursor: checkMeta[id]?.tip ? 'help' : undefined }}>
                    {checkMeta[id]?.tip ? (
                      <Tip text={checkMeta[id].tip}>{checkMeta[id]?.label ?? id}</Tip>
                    ) : (
                      checkMeta[id]?.label ?? id
                    )}
                  </td>
                  <td style={{ padding: '4px 8px', textAlign: 'center' }}>
                    {statusIcon[check.status] ?? check.status}
                  </td>
                  <td style={{ padding: '4px 0', color: colors.text }}>
                    {check.summary || ''}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Section>

        {repo.dep_syncs && repo.dep_syncs.length > 0 && (
          <Section title="Dependencies">
            {repo.dep_syncs.map(ds => (
              <div key={ds.module} style={{
                display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                padding: '4px 0', fontSize: 12,
                borderBottom: `1px solid ${colors.border}`,
              }}>
                <span style={{ color: colors.text }}>{ds.org}/{ds.repo}</span>
                <span style={{ color: ds.in_sync ? colors.green : colors.yellow, fontWeight: 600 }}>
                  {ds.in_sync ? 'synced' : 'behind'}
                </span>
              </div>
            ))}
          </Section>
        )}

        {repo.gomod_drifts && repo.gomod_drifts.length > 0 && (
          <Section title="Go.mod Drift">
            {repo.gomod_drifts.map(d => (
              <div key={d.module} style={{
                fontSize: 11, padding: '3px 0',
                borderBottom: `1px solid ${colors.border}`,
              }}>
                <div style={{ color: colors.muted, marginBottom: 1 }}>{d.module}</div>
                <span style={{ color: colors.yellow }}>{d.downstream}</span>
                <span style={{ color: colors.muted }}> → </span>
                <span style={{ color: colors.green }}>{d.upstream}</span>
              </div>
            ))}
          </Section>
        )}

        {repo.issues && repo.issues.length > 0 && (
          <Section title="Issues">
            {repo.issues.map((iss, i) => (
              <div key={i} style={{
                fontSize: 12, padding: '3px 0',
                color: iss.severity === 'error' ? colors.red : colors.yellow,
              }}>
                {iss.message}
              </div>
            ))}
          </Section>
        )}
      </div>
    </>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <h3 style={{
        fontSize: 11, fontWeight: 600, color: colors.muted,
        textTransform: 'uppercase', letterSpacing: '0.05em',
        marginBottom: 6,
      }}>
        {title}
      </h3>
      {children}
    </div>
  )
}

function Tip({ text, children }: { text: string; children: React.ReactNode }) {
  const [show, setShow] = useState(false)
  return (
    <span
      onMouseEnter={() => setShow(true)}
      onMouseLeave={() => setShow(false)}
      style={{ position: 'relative', cursor: 'help' }}
    >
      {children}
      {show && (
        <span style={{
          position: 'absolute',
          bottom: '100%',
          left: 0,
          marginBottom: 4,
          padding: '4px 8px',
          background: colors.surface2,
          border: `1px solid ${colors.border}`,
          borderRadius: 4,
          fontSize: 11,
          color: colors.text,
          whiteSpace: 'nowrap',
          zIndex: 10,
          pointerEvents: 'none',
        }}>
          {text}
        </span>
      )}
    </span>
  )
}

const linkButtonStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  padding: '4px 10px',
  borderRadius: 6,
  background: colors.surface2,
  border: `1px solid ${colors.border}`,
  color: colors.blue,
  fontSize: 12,
  fontWeight: 500,
  textDecoration: 'none',
}
