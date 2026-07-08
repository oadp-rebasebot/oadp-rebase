import { useMemo, useState, useCallback, useRef, useEffect } from 'react'
import type { RepoData, CvePR } from '../types'
import { classifyStatus, deriveLabel, deriveOrg } from '../dag-data'
import { getDepIds, getDependents } from '../layout'
import { colors, statusColors, orgColors } from '../styles/theme'
import { DetailPanel } from './DetailPanel'

interface DAGCanvasProps {
  repos: RepoData[]
  cvePRs: CvePR[]
}

interface Line {
  fromId: string
  toId: string
  inSync: boolean
}

export function DAGCanvas({ repos, cvePRs }: DAGCanvasProps) {
  const [selectedRepo, setSelectedRepo] = useState<RepoData | null>(null)
  const [hoveredRepo, setHoveredRepo] = useState<string | null>(null)
  const [lines, setLines] = useState<Line[]>([])
  const cardRefs = useRef<Map<string, HTMLDivElement>>(new Map())
  const containerRef = useRef<HTMLDivElement>(null)

  const repoSet = useMemo(() => new Set(repos.map(r => r.repo)), [repos])

  const columns = useMemo(() => {
    const waves = [...new Set(repos.map(r => r.wave))].sort((a, b) => a - b)
    return waves.map(w => ({
      wave: w,
      repos: repos.filter(r => r.wave === w).sort((a, b) => a.repo.localeCompare(b.repo)),
    }))
  }, [repos])

  const highlighted = useMemo(() => {
    if (!hoveredRepo) return new Set<string>()
    const repo = repos.find(r => r.repo === hoveredRepo)
    if (!repo) return new Set<string>()
    const depIds = getDepIds(repo, repoSet)
    const depentIds = getDependents(hoveredRepo, repos, repoSet)
    return new Set([hoveredRepo, ...depIds, ...depentIds])
  }, [hoveredRepo, repos, repoSet])

  const computeLines = useCallback(() => {
    if (!hoveredRepo || !containerRef.current) {
      setLines([])
      return
    }
    const repo = repos.find(r => r.repo === hoveredRepo)
    if (!repo) return

    const containerRect = containerRef.current.getBoundingClientRect()
    const scrollLeft = containerRef.current.scrollLeft
    const scrollTop = containerRef.current.scrollTop
    const newLines: Line[] = []

    for (const d of repo.dep_syncs ?? []) {
      const depId = `${d.org}/${d.repo}`
      if (!repoSet.has(depId)) continue

      const fromEl = cardRefs.current.get(depId)
      const toEl = cardRefs.current.get(hoveredRepo)
      if (!fromEl || !toEl) continue

      const fromRect = fromEl.getBoundingClientRect()
      const toRect = toEl.getBoundingClientRect()

      newLines.push({
        fromId: `${fromRect.right - containerRect.left + scrollLeft},${(fromRect.top + fromRect.bottom) / 2 - containerRect.top + scrollTop}`,
        toId: `${toRect.left - containerRect.left + scrollLeft},${(toRect.top + toRect.bottom) / 2 - containerRect.top + scrollTop}`,
        inSync: d.in_sync,
      })
    }

    const dependents = getDependents(hoveredRepo, repos, repoSet)
    for (const depId of dependents) {
      const depRepo = repos.find(r => r.repo === depId)
      if (!depRepo) continue
      const dep = depRepo.dep_syncs?.find(ds => `${ds.org}/${ds.repo}` === hoveredRepo)
      if (!dep) continue

      const fromEl = cardRefs.current.get(hoveredRepo)
      const toEl = cardRefs.current.get(depId)
      if (!fromEl || !toEl) continue

      const fromRect = fromEl.getBoundingClientRect()
      const toRect = toEl.getBoundingClientRect()

      newLines.push({
        fromId: `${fromRect.right - containerRect.left + scrollLeft},${(fromRect.top + fromRect.bottom) / 2 - containerRect.top + scrollTop}`,
        toId: `${toRect.left - containerRect.left + scrollLeft},${(toRect.top + toRect.bottom) / 2 - containerRect.top + scrollTop}`,
        inSync: dep.in_sync,
      })
    }

    setLines(newLines)
  }, [hoveredRepo, repos, repoSet])

  useEffect(() => {
    computeLines()
  }, [computeLines])

  const handleHover = useCallback((repoId: string | null) => {
    setHoveredRepo(repoId)
  }, [])

  const setCardRef = useCallback((id: string, el: HTMLDivElement | null) => {
    if (el) cardRefs.current.set(id, el)
    else cardRefs.current.delete(id)
  }, [])

  return (
    <div ref={containerRef} style={{ flex: 1, overflow: 'auto', padding: '24px 32px', position: 'relative' }}>
      <svg style={{
        position: 'absolute', top: 0, left: 0,
        width: '100%', height: '100%',
        pointerEvents: 'none', zIndex: 1,
      }}>
        {lines.map((line, i) => {
          const [x1, y1] = line.fromId.split(',').map(Number)
          const [x2, y2] = line.toId.split(',').map(Number)
          const midX = (x1 + x2) / 2
          return (
            <g key={i}>
              <path
                d={`M ${x1} ${y1} L ${midX} ${y1} L ${midX} ${y2} L ${x2} ${y2}`}
                fill="none"
                stroke={line.inSync ? colors.green : colors.yellow}
                strokeWidth={line.inSync ? 1.5 : 2.5}
                opacity={line.inSync ? 0.6 : 0.9}
                strokeDasharray={line.inSync ? undefined : '6 4'}
              />
              <polygon
                points={`${x2},${y2} ${x2 - 8},${y2 - 4} ${x2 - 8},${y2 + 4}`}
                fill={line.inSync ? colors.green : colors.yellow}
                opacity={line.inSync ? 0.6 : 0.9}
              />
            </g>
          )
        })}
      </svg>

      <div style={{ display: 'flex', gap: 28, minWidth: 'min-content', position: 'relative', zIndex: 2 }}>
        {columns.map(col => (
          <div key={col.wave} style={{ display: 'flex', flexDirection: 'column', gap: 12, minWidth: 260 }}>
            <div style={{
              fontSize: 11, fontWeight: 600, color: colors.muted,
              textTransform: 'uppercase', letterSpacing: '0.05em',
              padding: '0 2px 4px',
              borderBottom: `1px solid ${colors.border}`,
            }}>
              Wave {col.wave}
            </div>
            {col.repos.map(repo => (
              <RepoCard
                key={repo.repo}
                ref={el => setCardRef(repo.repo, el)}
                repo={repo}
                repoSet={repoSet}
                cvePRs={cvePRs}
                dimmed={hoveredRepo !== null && !highlighted.has(repo.repo)}
                onClick={() => setSelectedRepo(repo)}
                onHover={handleHover}
              />
            ))}
          </div>
        ))}
      </div>

      {selectedRepo && (
        <DetailPanel repo={selectedRepo} onClose={() => setSelectedRepo(null)} />
      )}
    </div>
  )
}

import { forwardRef } from 'react'

const RepoCard = forwardRef<HTMLDivElement, {
  repo: RepoData
  repoSet: Set<string>
  cvePRs: CvePR[]
  dimmed: boolean
  onClick: () => void
  onHover: (id: string | null) => void
}>(({ repo, repoSet, cvePRs, dimmed, onClick, onHover }, ref) => {
  const status = classifyStatus(repo)
  const sc = statusColors[status]
  const label = deriveLabel(repo.repo)
  const org = deriveOrg(repo.repo)
  const checks = repo.checks ?? {}

  const prCheck = checks.open_pr
  const prNum = prCheck?.summary?.replace('#', '')
  const prHref = prNum ? `https://github.com/${repo.repo}/pull/${prNum}` : undefined

  const depSummary = checks.dep_sync?.summary ?? ''
  const depStatus = checks.dep_sync?.status ?? 'na'

  const repoCvePRs = cvePRs.filter(p => `${p.org}/${p.repo}` === repo.repo)
  const internalDeps = (repo.dep_syncs ?? []).filter(d => repoSet.has(`${d.org}/${d.repo}`))

  const configOrg = repo.repo.replace('/', '_').replace(/-/g, '_')
  const configFile = `${configOrg}_${repo.branch}.env.sh`
  const configHref = `https://github.com/oadp-rebasebot/oadp-rebase/blob/oadp-dev/rebase-configs/${configFile}`

  return (
    <div
      ref={ref}
      onClick={onClick}
      onMouseEnter={() => onHover(repo.repo)}
      onMouseLeave={() => onHover(null)}
      style={{
        background: `linear-gradient(135deg, ${sc.bg}30, ${sc.bg}10)`,
        border: `1px solid ${sc.border}50`,
        borderRadius: 10,
        overflow: 'hidden',
        cursor: 'pointer',
        opacity: dimmed ? 0.15 : 1,
        transition: 'opacity 0.15s ease',
        boxShadow: `inset 0 1px 0 ${sc.border}20`,
      }}
    >

      <div style={{ padding: '8px 10px 7px' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 5 }}>
          <span style={{ fontWeight: 600, fontSize: 13, color: colors.text }}>{label}</span>
          <span style={{
            fontSize: 9, fontWeight: 600, padding: '1px 6px', borderRadius: 10,
            background: `${orgColors[org] ?? colors.muted}20`,
            color: orgColors[org] ?? colors.muted,
            textTransform: 'uppercase',
          }}>
            {org}
          </span>
        </div>

        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3, marginBottom: 5 }}>
          {prCheck?.status === 'ok' && prHref && (
            <Badge color={colors.blue} href={prHref}>PR #{prNum}</Badge>
          )}
          {depSummary && (
            <Badge color={depStatus === 'ok' ? colors.green : depStatus === 'fail' ? colors.yellow : colors.muted}>
              Deps {depSummary}
            </Badge>
          )}
          {repoCvePRs.length > 0 && (
            <Badge color={colors.red} href={repoCvePRs[0].url}>CVE #{repoCvePRs[0].number}</Badge>
          )}
        </div>

        {internalDeps.length > 0 && (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '3px 8px', marginBottom: 5 }}>
            {internalDeps.map(d => {
              const commitCount = d.commits?.length ?? 0
              return (
                <span key={`${d.org}/${d.repo}`} style={{
                  fontSize: 10, lineHeight: '16px',
                  color: d.in_sync ? colors.green : colors.yellow,
                }}>
                  {d.in_sync ? '✓' : '✗'}{' '}
                  <span style={{ color: colors.muted }}>{d.repo}</span>
                  {!d.in_sync && commitCount > 0 && (
                    <span style={{ color: colors.yellow, fontWeight: 600 }}> ({commitCount})</span>
                  )}
                </span>
              )
            })}
          </div>
        )}

        <div style={{ fontSize: 10, color: colors.muted, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {repo.upstream ? `← ${repo.upstream.split(' @ ')[0]}` : 'hooks-only'}
        </div>
      </div>
    </div>
  )
})

RepoCard.displayName = 'RepoCard'

function Badge({ color, href, children }: { color: string; href?: string; children: React.ReactNode }) {
  const style: React.CSSProperties = {
    display: 'inline-flex', alignItems: 'center', gap: 3,
    padding: '1px 6px', borderRadius: 4,
    background: `${color}18`, fontSize: 10, lineHeight: '16px',
    color, fontWeight: 600, whiteSpace: 'nowrap', textDecoration: 'none',
  }
  if (href) {
    return <a href={href} target="_blank" rel="noopener noreferrer" onClick={e => e.stopPropagation()} style={style}>{children}</a>
  }
  return <span style={style}>{children}</span>
}
