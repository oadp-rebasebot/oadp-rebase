export interface CheckResult {
  status: 'ok' | 'fail' | 'warn' | 'skip' | 'na'
  summary: string
}

export interface DepSync {
  module: string
  org: string
  repo: string
  have: string
  head: string
  in_sync: boolean
  commits?: { sha: string; message: string }[]
}

export interface GoModDrift {
  module: string
  downstream: string
  upstream: string
}

export interface Issue {
  severity: 'error' | 'warning'
  message: string
}

export interface RepoData {
  repo: string
  branch: string
  wave: number
  skip: boolean
  upstream?: string
  checks: Record<string, CheckResult>
  dep_syncs?: DepSync[]
  gomod_drifts?: GoModDrift[]
  issues?: Issue[]
}

export interface DAGMetadata {
  branches: string[]
  generated_at: string
  default_branch: string
}

export interface CvePR {
  org: string
  repo: string
  number: number
  title: string
  url: string
  created: string
}

export interface BranchData {
  repos: RepoData[]
  cve_prs: CvePR[]
}

export type RepoStatus = 'done' | 'prog' | 'need' | 'skip'
