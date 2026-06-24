import { strict as assert } from 'node:assert';
import { describe, it } from 'node:test';
import { classifyStatus, deriveLabel, transformData } from './dag-data.js';

describe('classifyStatus', () => {
  it('returns "skip" when repo is skipped', () => {
    assert.equal(classifyStatus({ skip: true, checks: {} }), 'skip');
  });

  it('returns "prog" when open PR exists', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'ok', summary: '#42' },
        dep_sync: { status: 'fail', summary: '1/2' },
      },
    };
    assert.equal(classifyStatus(repo), 'prog');
  });

  it('returns "need" when deps are out of sync and no PR', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'fail', summary: '1/3' },
      },
    };
    assert.equal(classifyStatus(repo), 'need');
  });

  it('returns "done" when deps in sync and no PR', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'ok', summary: '3/3' },
      },
    };
    assert.equal(classifyStatus(repo), 'done');
  });

  it('returns "done" when no dep_sync check exists (wave 1, no deps)', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
      },
    };
    assert.equal(classifyStatus(repo), 'done');
  });

  it('returns "done" for dep_sync status "na"', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'na', summary: '' },
      },
    };
    assert.equal(classifyStatus(repo), 'done');
  });

  it('returns "need" for dep_sync status "warn"', () => {
    const repo = {
      skip: false,
      checks: {
        open_pr: { status: 'fail', summary: '' },
        dep_sync: { status: 'warn', summary: '2/3' },
      },
    };
    assert.equal(classifyStatus(repo), 'need');
  });
});

describe('deriveLabel', () => {
  it('strips org prefix', () => {
    assert.equal(deriveLabel('openshift/velero'), 'velero');
  });

  it('handles plugin names', () => {
    assert.equal(deriveLabel('openshift/velero-plugin-for-aws'), 'velero-plugin-for-aws');
  });

  it('handles single-segment name', () => {
    assert.equal(deriveLabel('velero'), 'velero');
  });
});

describe('transformData', () => {
  const sampleData = [
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
  ];

  it('creates wave group nodes and repo nodes', () => {
    const { nodes } = transformData(sampleData);
    const waveNodes = nodes.filter(n => n.classes === 'wave-group');
    const repoNodes = nodes.filter(n => !n.classes);
    assert.equal(waveNodes.length, 3);
    assert.equal(repoNodes.length, 3);
  });

  it('assigns repos to wave parent nodes', () => {
    const { nodes } = transformData(sampleData);
    const velero = nodes.find(n => n.data.id === 'openshift/velero');
    assert.equal(velero.data.parent, 'wave-2');
  });

  it('assigns correct labels', () => {
    const { nodes } = transformData(sampleData);
    const repoLabels = nodes.filter(n => !n.classes).map(n => n.data.label);
    assert.deepEqual(repoLabels, ['kopia', 'velero', 'velero-plugin-for-aws']);
  });

  it('assigns correct statuses', () => {
    const { nodes } = transformData(sampleData);
    const statuses = nodes.filter(n => !n.classes).map(n => n.data.status);
    assert.deepEqual(statuses, ['done', 'done', 'prog']);
  });

  it('creates edges from dep_syncs', () => {
    const { edges } = transformData(sampleData);
    assert.equal(edges.length, 2);
  });

  it('edges point from dependency to dependent', () => {
    const { edges } = transformData(sampleData);
    const kopiaToVelero = edges.find(e => e.data.source === 'migtools/kopia');
    assert.ok(kopiaToVelero);
    assert.equal(kopiaToVelero.data.target, 'openshift/velero');
  });

  it('tracks in_sync on edges', () => {
    const { edges } = transformData(sampleData);
    const veleroToAws = edges.find(e => e.data.target === 'openshift/velero-plugin-for-aws');
    assert.ok(veleroToAws);
    assert.equal(veleroToAws.data.inSync, false);
  });

  it('filters out edges to repos not in the dataset', () => {
    const dataWithExternalDep = [
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
    ];
    const { edges } = transformData(dataWithExternalDep);
    assert.equal(edges.length, 0);
  });

  it('handles repos with no dep_syncs', () => {
    const data = [
      {
        repo: 'migtools/kopia',
        branch: 'oadp-1.6',
        wave: 1,
        skip: false,
        checks: { dep_sync: { status: 'ok', summary: '' }, open_pr: { status: 'fail', summary: '' } },
      },
    ];
    const { nodes, edges } = transformData(data);
    const repoNodes = nodes.filter(n => !n.classes);
    assert.equal(repoNodes.length, 1);
    assert.equal(edges.length, 0);
  });

  it('handles empty input', () => {
    const { nodes, edges } = transformData([]);
    assert.equal(nodes.length, 0);
    assert.equal(edges.length, 0);
  });

  it('skipped repos still appear as nodes', () => {
    const data = [
      {
        repo: 'openshift/restic',
        branch: 'oadp-1.5',
        wave: 1,
        skip: true,
        checks: { open_pr: { status: 'fail', summary: '' } },
      },
    ];
    const { nodes } = transformData(data);
    const repoNodes = nodes.filter(n => !n.classes);
    assert.equal(repoNodes.length, 1);
    assert.equal(repoNodes[0].data.status, 'skip');
  });

  it('preserves full repo data for tooltips', () => {
    const { nodes } = transformData(sampleData);
    const kopia = nodes.find(n => n.data.id === 'migtools/kopia');
    assert.equal(kopia.data.fullData.repo, 'migtools/kopia');
    assert.equal(kopia.data.fullData.wave, 1);
  });

  it('deduplicates edges between same source and target', () => {
    const data = [
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
    ];
    const { edges } = transformData(data);
    assert.equal(edges.length, 1);
  });
});
