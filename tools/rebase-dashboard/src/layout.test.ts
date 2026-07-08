import { describe, it, expect } from 'vitest'
import { computeDepth, getDepIds, getDependents } from './layout'
import type { RepoData } from './types'

function repo(name: string, wave: number, depSyncs?: { org: string; repo: string }[]): RepoData {
  return {
    repo: name,
    branch: 'oadp-1.6',
    wave,
    skip: false,
    checks: {},
    dep_syncs: depSyncs?.map(d => ({
      module: `github.com/${d.org}/${d.repo}`,
      org: d.org,
      repo: d.repo,
      have: 'abc',
      head: 'abc',
      in_sync: true,
    })),
  }
}

describe('computeDepth', () => {
  it('assigns depth 0 to repos with no deps', () => {
    const repos = [repo('migtools/kopia', 1)]
    const depth = computeDepth(repos)
    expect(depth.get('migtools/kopia')).toBe(0)
  })

  it('assigns depth 1 to repos depending on depth-0 repos', () => {
    const repos = [
      repo('migtools/kopia', 1),
      repo('openshift/velero', 2, [{ org: 'migtools', repo: 'kopia' }]),
    ]
    const depth = computeDepth(repos)
    expect(depth.get('migtools/kopia')).toBe(0)
    expect(depth.get('openshift/velero')).toBe(1)
  })

  it('computes transitive depth correctly', () => {
    const repos = [
      repo('migtools/kopia', 1),
      repo('openshift/velero', 2, [{ org: 'migtools', repo: 'kopia' }]),
      repo('openshift/velero-plugin-for-aws', 3, [{ org: 'openshift', repo: 'velero' }]),
      repo('openshift/oadp-must-gather', 5, [{ org: 'openshift', repo: 'velero-plugin-for-aws' }]),
    ]
    const depth = computeDepth(repos)
    expect(depth.get('migtools/kopia')).toBe(0)
    expect(depth.get('openshift/velero')).toBe(1)
    expect(depth.get('openshift/velero-plugin-for-aws')).toBe(2)
    expect(depth.get('openshift/oadp-must-gather')).toBe(3)
  })

  it('uses max depth when multiple deps at different depths', () => {
    const repos = [
      repo('migtools/kopia', 1),
      repo('openshift/velero', 2, [{ org: 'migtools', repo: 'kopia' }]),
      repo('openshift/oadp-operator', 3, [
        { org: 'migtools', repo: 'kopia' },
        { org: 'openshift', repo: 'velero' },
      ]),
    ]
    const depth = computeDepth(repos)
    expect(depth.get('openshift/oadp-operator')).toBe(2)
  })

  it('ignores deps not in the repo set', () => {
    const repos = [
      repo('openshift/velero', 2, [{ org: 'some', repo: 'external' }]),
    ]
    const depth = computeDepth(repos)
    expect(depth.get('openshift/velero')).toBe(0)
  })

  it('handles circular deps gracefully', () => {
    const repos = [
      repo('a/one', 1, [{ org: 'a', repo: 'two' }]),
      repo('a/two', 1, [{ org: 'a', repo: 'one' }]),
    ]
    const depth = computeDepth(repos)
    expect(depth.get('a/one')).toBeDefined()
    expect(depth.get('a/two')).toBeDefined()
  })

  it('handles empty input', () => {
    const depth = computeDepth([])
    expect(depth.size).toBe(0)
  })
})

describe('getDepIds', () => {
  it('returns dep IDs that exist in the repo set', () => {
    const r = repo('openshift/velero', 2, [
      { org: 'migtools', repo: 'kopia' },
      { org: 'some', repo: 'external' },
    ])
    const repoSet = new Set(['migtools/kopia', 'openshift/velero'])
    const ids = getDepIds(r, repoSet)
    expect(ids).toEqual(new Set(['migtools/kopia']))
  })

  it('returns empty set when no deps', () => {
    const r = repo('migtools/kopia', 1)
    const ids = getDepIds(r, new Set(['migtools/kopia']))
    expect(ids.size).toBe(0)
  })
})

describe('getDependents', () => {
  it('finds repos that depend on the given repo', () => {
    const repos = [
      repo('migtools/kopia', 1),
      repo('openshift/velero', 2, [{ org: 'migtools', repo: 'kopia' }]),
      repo('openshift/oadp-operator', 3, [{ org: 'migtools', repo: 'kopia' }]),
      repo('openshift/velero-plugin-for-aws', 3, [{ org: 'openshift', repo: 'velero' }]),
    ]
    const repoSet = new Set(repos.map(r => r.repo))
    const deps = getDependents('migtools/kopia', repos, repoSet)
    expect(deps).toEqual(new Set(['openshift/velero', 'openshift/oadp-operator']))
  })

  it('returns empty set when nothing depends on the repo', () => {
    const repos = [
      repo('migtools/kopia', 1),
      repo('openshift/velero', 2, [{ org: 'migtools', repo: 'kopia' }]),
    ]
    const repoSet = new Set(repos.map(r => r.repo))
    const deps = getDependents('openshift/velero', repos, repoSet)
    expect(deps.size).toBe(0)
  })
})
