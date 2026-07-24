import type { RepoData, RepoStatus } from './types'

export function classifyStatus(repo: RepoData): RepoStatus {
  if (repo.skip) return 'skip'
  if (repo.checks?.open_pr?.status === 'ok') return 'prog'
  const depStatus = repo.checks?.dep_sync?.status
  if (depStatus === 'fail' || depStatus === 'warn') return 'need'
  return 'done'
}

export function deriveLabel(repoFullName: string): string {
  const parts = repoFullName.split('/')
  return parts.length > 1 ? parts.slice(1).join('/') : repoFullName
}

export function deriveOrg(repoFullName: string): string {
  return repoFullName.split('/')[0] ?? ''
}

export function isPlugin(repoFullName: string): boolean {
  return deriveLabel(repoFullName).includes('plugin')
}

interface CytoscapeNode {
  data: Record<string, unknown>
  classes?: string
}

interface CytoscapeEdge {
  data: { id: string; source: string; target: string; inSync: boolean; module: string }
}

export function transformData(repos: RepoData[]): { nodes: CytoscapeNode[]; edges: CytoscapeEdge[] } {
  const repoIds = new Set(repos.map(r => r.repo))

  const waves = [...new Set(repos.map(r => r.wave))].sort((a, b) => a - b)
  const waveNodes: CytoscapeNode[] = waves.map(w => ({
    data: { id: `wave-${w}`, label: `Wave ${w}` },
    classes: 'wave-group',
  }))

  const nodes: CytoscapeNode[] = [
    ...waveNodes,
    ...repos.map(r => ({
      data: {
        id: r.repo,
        label: deriveLabel(r.repo),
        wave: r.wave,
        parent: `wave-${r.wave}`,
        status: classifyStatus(r),
        prSummary: r.checks?.open_pr?.summary || '',
        fullData: r,
      },
    })),
  ]

  const edgeSet = new Set<string>()
  const edges: CytoscapeEdge[] = []
  for (const r of repos) {
    for (const dep of r.dep_syncs || []) {
      const sourceId = `${dep.org}/${dep.repo}`
      if (!repoIds.has(sourceId)) continue
      const edgeKey = `${sourceId}->${r.repo}`
      if (edgeSet.has(edgeKey)) continue
      edgeSet.add(edgeKey)
      edges.push({
        data: {
          id: edgeKey,
          source: sourceId,
          target: r.repo,
          inSync: dep.in_sync,
          module: dep.module,
        },
      })
    }
  }

  return { nodes, edges }
}
