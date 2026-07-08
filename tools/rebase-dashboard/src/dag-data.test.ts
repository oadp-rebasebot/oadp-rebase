import { describe, it, expect } from 'vitest'
import { classifyStatus, deriveLabel, transformData } from './dag-data'
import type { RepoData } from './types'

describe('classifyStatus', () => {
  it('returns "skip" when repo is skipped', () => {
    expect(classifyStatus({ skip: true, checks: {} } as RepoData)).toBe('skip')
  })

  it('returns "prog" when open PR exists', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'ok', summary: '#42' },
        dep_sync: { status: 'fail', summary: '1/2' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('prog')
  })

  it('returns "need" when deps are out of sync and no PR', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'fail', summary: '1/3' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('need')
  })

  it('returns "done" when deps in sync and no PR', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'ok', summary: '3/3' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('done')
  })

  it('returns "done" when no dep_sync check exists (wave 1, no deps)', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('done')
  })

  it('returns "done" for dep_sync status "na"', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'na', summary: '' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('done')
  })

  it('returns "need" for dep_sync status "warn"', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'warn', summary: '2/3' },
      },
    } as RepoData
    expect(classifyStatus(repo)).toBe('need')
  })
})

describe('deriveLabel', () => {
  it('strips org prefix', () => {
    expect(deriveLabel('openshift/velero')).toBe('velero')
  })

  it('handles plugin names', () => {
    expect(deriveLabel('openshift/velero-plugin-for-aws')).toBe('velero-plugin-for-aws')
  })

  it('handles single-segment name', () => {
    expect(deriveLabel('velero')).toBe('velero')
  })
})

describe('transformData', () => {
  const sampleData: RepoData[] = [
    {
      repo: 'migtools/kopia',
      branch: 'oadp-1.6',
      wave: 1,
      skip: false,
      checks: {
        dep_sync: { status: 'ok', summary: '' },
        open_pr: { status: 'fail', summary: '' },
      },
    },
    {
      repo: 'openshift/velero',
      branch: 'oadp-1.6',
      wave: 2,
      skip: false,
      checks: {
        dep_sync: { status: 'ok', summary: '1/1' },
        open_pr: { status: 'fail', summary: '' },
      },
      dep_syncs: [
        { module: 'github.com/kopia/kopia', org: 'migtools', repo: 'kopia', have: 'abc123', head: 'abc123', in_sync: true },
      ],
    },
    {
      repo: 'openshift/velero-plugin-for-aws',
      branch: 'oadp-1.6',
      wave: 3,
      skip: false,
      checks: {
        dep_sync: { status: 'fail', summary: '0/1' },
        open_pr: { status: 'ok', summary: '#55' },
      },
      dep_syncs: [
        { module: 'github.com/vmware-tanzu/velero', org: 'openshift', repo: 'velero', have: 'old123', head: 'new456', in_sync: false },
      ],
    },
  ]

  it('creates wave group nodes and repo nodes', () => {
    const { nodes } = transformData(sampleData)
    const waveNodes = nodes.filter(n => n.classes === 'wave-group')
    const repoNodes = nodes.filter(n => !n.classes)
    expect(waveNodes).toHaveLength(3)
    expect(repoNodes).toHaveLength(3)
  })

  it('assigns repos to wave parent nodes', () => {
    const { nodes } = transformData(sampleData)
    const velero = nodes.find(n => n.data.id === 'openshift/velero')
    expect(velero?.data.parent).toBe('wave-2')
  })

  it('assigns correct labels', () => {
    const { nodes } = transformData(sampleData)
    const repoLabels = nodes.filter(n => !n.classes).map(n => n.data.label)
    expect(repoLabels).toEqual(['kopia', 'velero', 'velero-plugin-for-aws'])
  })

  it('assigns correct statuses', () => {
    const { nodes } = transformData(sampleData)
    const statuses = nodes.filter(n => !n.classes).map(n => n.data.status)
    expect(statuses).toEqual(['done', 'done', 'prog'])
  })

  it('creates edges from dep_syncs', () => {
    const { edges } = transformData(sampleData)
    expect(edges).toHaveLength(2)
  })

  it('edges point from dependency to dependent', () => {
    const { edges } = transformData(sampleData)
    const kopiaToVelero = edges.find(e => e.data.source === 'migtools/kopia')
    expect(kopiaToVelero).toBeTruthy()
    expect(kopiaToVelero?.data.target).toBe('openshift/velero')
  })

  it('tracks in_sync on edges', () => {
    const { edges } = transformData(sampleData)
    const veleroToAws = edges.find(e => e.data.target === 'openshift/velero-plugin-for-aws')
    expect(veleroToAws).toBeTruthy()
    expect(veleroToAws?.data.inSync).toBe(false)
  })

  it('filters out edges to repos not in the dataset', () => {
    const dataWithExternalDep: RepoData[] = [
      {
        repo: 'openshift/velero',
        branch: 'oadp-1.6',
        wave: 2,
        skip: false,
        checks: { dep_sync: { status: 'ok', summary: '' }, open_pr: { status: 'fail', summary: '' } },
        dep_syncs: [
          { module: 'github.com/some/external', org: 'some', repo: 'external', have: 'a', head: 'a', in_sync: true },
        ],
      },
    ]
    const { edges } = transformData(dataWithExternalDep)
    expect(edges).toHaveLength(0)
  })

  it('handles repos with no dep_syncs', () => {
    const data: RepoData[] = [
      {
        repo: 'migtools/kopia',
        branch: 'oadp-1.6',
        wave: 1,
        skip: false,
        checks: { dep_sync: { status: 'ok', summary: '' }, open_pr: { status: 'fail', summary: '' } },
      },
    ]
    const { nodes, edges } = transformData(data)
    const repoNodes = nodes.filter(n => !n.classes)
    expect(repoNodes).toHaveLength(1)
    expect(edges).toHaveLength(0)
  })

  it('handles empty input', () => {
    const { nodes, edges } = transformData([])
    expect(nodes).toHaveLength(0)
    expect(edges).toHaveLength(0)
  })

  it('skipped repos still appear as nodes', () => {
    const data: RepoData[] = [
      {
        repo: 'openshift/restic',
        branch: 'oadp-1.5',
        wave: 1,
        skip: true,
        checks: { open_pr: { status: 'fail', summary: '' } },
      },
    ]
    const { nodes } = transformData(data)
    const repoNodes = nodes.filter(n => !n.classes)
    expect(repoNodes).toHaveLength(1)
    expect(repoNodes[0].data.status).toBe('skip')
  })

  it('preserves full repo data for tooltips', () => {
    const { nodes } = transformData(sampleData)
    const kopia = nodes.find(n => n.data.id === 'migtools/kopia')
    expect((kopia?.data.fullData as RepoData).repo).toBe('migtools/kopia')
    expect((kopia?.data.fullData as RepoData).wave).toBe(1)
  })

  it('deduplicates edges between same source and target', () => {
    const data: RepoData[] = [
      {
        repo: 'openshift/oadp-operator',
        branch: 'oadp-1.6',
        wave: 3,
        skip: false,
        checks: { dep_sync: { status: 'fail', summary: '' }, open_pr: { status: 'fail', summary: '' } },
        dep_syncs: [
          { module: 'github.com/vmware-tanzu/velero', org: 'openshift', repo: 'velero', have: 'a', head: 'b', in_sync: false },
          { module: 'github.com/openshift/velero', org: 'openshift', repo: 'velero', have: 'a', head: 'b', in_sync: false },
        ],
      },
      {
        repo: 'openshift/velero',
        branch: 'oadp-1.6',
        wave: 2,
        skip: false,
        checks: { dep_sync: { status: 'ok', summary: '' }, open_pr: { status: 'fail', summary: '' } },
      },
    ]
    const { edges } = transformData(data)
    expect(edges).toHaveLength(1)
  })
})
