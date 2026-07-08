import { useState, useEffect, useCallback } from 'react'
import type { DAGMetadata, RepoData, CvePR } from '../types'

interface DAGState {
  metadata: DAGMetadata | null
  repos: RepoData[]
  cvePRs: CvePR[]
  branch: string
  loading: boolean
}

export function useDAGData() {
  const [state, setState] = useState<DAGState>({
    metadata: null,
    repos: [],
    cvePRs: [],
    branch: '',
    loading: true,
  })

  useEffect(() => {
    (async () => {
      try {
        const resp = await fetch('./dag-metadata.json')
        if (!resp.ok) throw new Error(resp.statusText)
        const meta: DAGMetadata = await resp.json()
        setState(s => ({ ...s, metadata: meta, branch: meta.default_branch }))
      } catch {
        setState(s => ({ ...s, loading: false }))
      }
    })()
  }, [])

  useEffect(() => {
    if (!state.branch) return
    let stale = false
    setState(s => ({ ...s, loading: true }))
    ;(async () => {
      try {
        const resp = await fetch(`./dag-${state.branch}.json`)
        if (!resp.ok) throw new Error(resp.statusText)
        const raw = await resp.json()
        if (stale) return
        const repos: RepoData[] = Array.isArray(raw) ? raw : (raw.repos ?? [])
        const cvePRs: CvePR[] = Array.isArray(raw) ? [] : (raw.cve_prs ?? [])
        setState(s => ({ ...s, repos, cvePRs, loading: false }))
      } catch {
        if (stale) return
        setState(s => ({ ...s, repos: [], cvePRs: [], loading: false }))
      }
    })()
    return () => { stale = true }
  }, [state.branch])

  const setBranch = useCallback((branch: string) => {
    setState(s => ({ ...s, branch }))
  }, [])

  return { ...state, setBranch }
}
