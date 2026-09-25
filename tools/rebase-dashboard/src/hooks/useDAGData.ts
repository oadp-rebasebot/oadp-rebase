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
  const remoteMetadataRevisionRef = useRef<string | null>(null)
  const remoteSnapshotRevisionRef = useRef<string | null>(null)

  async function fetchRemoteJSON<T>(file: string): Promise<T> {
    const response = await fetch(dataURL(dashboardDataURL, file), { cache: 'no-store' })
    if (!response.ok) throw new Error(response.statusText)
    return response.json() as Promise<T>
  }

  async function fetchInitialJSON<T>(file: string, bundledFile: string): Promise<{ data: T; remote: boolean }> {
    try {
      return { data: await fetchRemoteJSON<T>(file), remote: true }
    } catch {
      const response = await fetch(`./${bundledFile}`)
      if (!response.ok) throw new Error(response.statusText)
      return { data: await response.json() as T, remote: false }
    }
  }

  useEffect(() => {
    (async () => {
      try {
        const { data: meta, remote } = await fetchInitialJSON<DAGMetadata>('metadata.json', 'dag-metadata.json')
        remoteMetadataRevisionRef.current = remote ? metadataRevision(meta) : null
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
    remoteSnapshotRevisionRef.current = null
    setState(s => ({ ...s, loading: true }))
    ;(async () => {
      try {
        const { data: raw, remote } = await fetchInitialJSON<unknown>(`dag-${state.branch}.json`, `dag-${state.branch}.json`)
        if (stale) return
        const { repos, cvePRs } = parseSnapshot(raw)
        if (remote) remoteSnapshotRevisionRef.current = remoteMetadataRevisionRef.current
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
        const metadata = await fetchRemoteJSON<DAGMetadata>('metadata.json')
        const revision = metadataRevision(metadata)
        if (stale || remoteSnapshotRevisionRef.current === revision) return

        const raw = await fetchRemoteJSON<unknown>(`dag-${state.branch}.json`)
        if (stale) return
        const { repos, cvePRs } = parseSnapshot(raw)
        remoteMetadataRevisionRef.current = revision
        remoteSnapshotRevisionRef.current = revision
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
