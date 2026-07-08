import type { RepoData } from './types'

export function computeDepth(repos: RepoData[]): Map<string, number> {
  const repoSet = new Set(repos.map(r => r.repo))
  const deps = new Map<string, string[]>()
  for (const r of repos) {
    const myDeps = (r.dep_syncs ?? [])
      .map(d => `${d.org}/${d.repo}`)
      .filter(id => repoSet.has(id))
    deps.set(r.repo, myDeps)
  }

  const depth = new Map<string, number>()
  function resolve(id: string, visited: Set<string>): number {
    if (depth.has(id)) return depth.get(id)!
    if (visited.has(id)) return 0
    visited.add(id)
    const myDeps = deps.get(id) ?? []
    const d = myDeps.length === 0 ? 0 : Math.max(...myDeps.map(dep => resolve(dep, visited))) + 1
    depth.set(id, d)
    return d
  }

  for (const r of repos) resolve(r.repo, new Set())
  return depth
}

export function getDepIds(repo: RepoData, repoSet: Set<string>): Set<string> {
  const ids = new Set<string>()
  for (const d of repo.dep_syncs ?? []) {
    const id = `${d.org}/${d.repo}`
    if (repoSet.has(id)) ids.add(id)
  }
  return ids
}

export function getDependents(repoId: string, repos: RepoData[], repoSet: Set<string>): Set<string> {
  const ids = new Set<string>()
  for (const r of repos) {
    for (const d of r.dep_syncs ?? []) {
      if (`${d.org}/${d.repo}` === repoId && repoSet.has(r.repo)) {
        ids.add(r.repo)
      }
    }
  }
  return ids
}
