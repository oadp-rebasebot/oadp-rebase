import { useState, useEffect, useCallback, useRef } from 'react'
import type { CvePR, DAGMetadata, RepoData } from '../types'
import { branchFromSearch } from '../branch'
import { dashboardDataURL, dataURL, metadataRevision, parseSnapshot } from '../data-source'

const refreshIntervalMS = 15_000

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
  const metadataRef = useRef<DAGMetadata | null>(null)

  useEffect(() => {
    metadataRef.current = state.metadata
  }, [state.metadata])

  async function fetchJSON<T>(file: string, bundledFile: string): Promise<T> {
    try {
      const response = await fetch(dataURL(dashboardDataURL, file), { cache: 'no-store' })
      if (!response.ok) throw new Error(response.statusText)
      return response.json() as Promise<T>
    } catch {
      const response = await fetch(`./${bundledFile}`)
      if (!response.ok) throw new Error(response.statusText)
      return response.json() as Promise<T>
    }
  }

  useEffect(() => {
    (async () => {
      try {
        const meta = await fetchJSON<DAGMetadata>('metadata.json', 'dag-metadata.json')
        setState(s => ({
          ...s,
          metadata: meta,
          branch: branchFromSearch(window.location.search, meta.branches, meta.default_branch),
        }))
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
        const raw = await fetchJSON<unknown>(`dag-${state.branch}.json`, `dag-${state.branch}.json`)
        if (stale) return
        const { repos, cvePRs } = parseSnapshot(raw)
        setState(s => ({ ...s, repos, cvePRs, loading: false }))
      } catch {
        if (stale) return
        setState(s => ({ ...s, repos: [], cvePRs: [], loading: false }))
      }
    })()
    return () => { stale = true }
  }, [state.branch])

  useEffect(() => {
    if (!state.branch) return
    let stale = false

    const refresh = async () => {
      try {
        const metadata = await fetchJSON<DAGMetadata>('metadata.json', 'dag-metadata.json')
        if (stale || metadataRevision(metadata) === metadataRevision(metadataRef.current ?? metadata)) return

        const raw = await fetchJSON<unknown>(`dag-${state.branch}.json`, `dag-${state.branch}.json`)
        if (stale) return
        const { repos, cvePRs } = parseSnapshot(raw)
        setState(s => ({ ...s, metadata, repos, cvePRs }))
      } catch {
        // Keep the last known-good snapshot when a refresh fails.
      }
    }

    const interval = window.setInterval(refresh, refreshIntervalMS)
    return () => {
      stale = true
      window.clearInterval(interval)
    }
  }, [state.branch])

  const setBranch = useCallback((branch: string) => {
    const url = new URL(window.location.href)
    url.searchParams.set('branch', branch)
    window.history.replaceState(null, '', url)
    setState(s => ({ ...s, branch }))
  }, [])

  return { ...state, setBranch }
}
