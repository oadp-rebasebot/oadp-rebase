import { describe, expect, it } from 'vitest'
import { dataURL, metadataRevision, parseSnapshot } from './data-source'

describe('dataURL', () => {
  it('joins the data base, file, and cache-busting revision', () => {
    expect(dataURL('https://example.com/data/', 'metadata.json', 42))
      .toBe('https://example.com/data/metadata.json?v=42')
  })
})

describe('metadataRevision', () => {
  it('prefers an explicit revision', () => {
    expect(metadataRevision({ branches: [], default_branch: '', generated_at: 'old', revision: 'new' })).toBe('new')
  })

  it('uses the generation timestamp for bundled legacy data', () => {
    expect(metadataRevision({ branches: [], default_branch: '', generated_at: 'old' })).toBe('old')
  })
})

describe('parseSnapshot', () => {
  it('supports legacy repository arrays', () => {
    expect(parseSnapshot([{ repo: 'openshift/velero' }])).toEqual({
      repos: [{ repo: 'openshift/velero' }],
      cvePRs: [],
    })
  })

  it('supports snapshots with CVE PRs', () => {
    expect(parseSnapshot({ repos: [], cve_prs: [{ number: 42 }] })).toEqual({
      repos: [],
      cvePRs: [{ number: 42 }],
    })
  })
})
