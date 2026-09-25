import type { CvePR, DAGMetadata, RepoData } from './types'

export const dashboardDataURL = import.meta.env.VITE_DASHBOARD_DATA_URL
  ?? 'https://raw.githubusercontent.com/oadp-rebasebot/oadp-rebase/dashboard-data'

export function dataURL(baseURL: string, file: string, cacheBuster = Date.now()): string {
  return `${baseURL.replace(/\/$/, '')}/${file}?v=${cacheBuster}`
}

export function metadataRevision(metadata: DAGMetadata): string {
  return metadata.revision ?? metadata.generated_at
}

export function parseSnapshot(raw: unknown): { repos: RepoData[]; cvePRs: CvePR[] } {
  if (Array.isArray(raw)) return { repos: raw as RepoData[], cvePRs: [] }
  const snapshot = raw as { repos?: RepoData[]; cve_prs?: CvePR[] }
  return { repos: snapshot.repos ?? [], cvePRs: snapshot.cve_prs ?? [] }
}
