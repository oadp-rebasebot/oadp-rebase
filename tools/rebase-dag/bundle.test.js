import { strict as assert } from 'node:assert';
import { describe, it } from 'node:test';
import { classifyStatus, transformData } from './dag-data.js';

/**
 * Tests that verify the dag-bundle.json contract between the workflow
 * (which generates the bundle) and the page (which consumes it).
 */

const sampleBundle = {
  branches: ['oadp-1.6', 'oadp-1.5'],
  generated_at: '2026-06-24T15:51:15Z',
  default_branch: 'oadp-1.6',
  data: {
    'oadp-1.6': [
      {
        repo: 'migtools/kopia', branch: 'oadp-1.6', wave: 1, skip: false,
        checks: { open_pr: { status: 'na', summary: '' }, dep_sync: { status: 'ok', summary: '' } },
      },
      {
        repo: 'openshift/velero', branch: 'oadp-1.6', wave: 2, skip: false,
        checks: { open_pr: { status: 'ok', summary: '#42' }, dep_sync: { status: 'ok', summary: '1/1' } },
        dep_syncs: [{ module: 'github.com/kopia/kopia', org: 'migtools', repo: 'kopia', have: 'abc', head: 'abc', in_sync: true }],
      },
    ],
    'oadp-1.5': [
      {
        repo: 'migtools/kopia', branch: 'oadp-1.5', wave: 1, skip: false,
        checks: { open_pr: { status: 'na', summary: '' }, dep_sync: { status: 'ok', summary: '' } },
      },
    ],
  },
};

describe('bundle contract', () => {
  it('has required top-level fields', () => {
    assert.ok(Array.isArray(sampleBundle.branches));
    assert.ok(sampleBundle.branches.length > 0);
    assert.ok(typeof sampleBundle.generated_at === 'string');
    assert.ok(typeof sampleBundle.default_branch === 'string');
    assert.ok(typeof sampleBundle.data === 'object');
  });

  it('default_branch exists in branches list', () => {
    assert.ok(sampleBundle.branches.includes(sampleBundle.default_branch));
  });

  it('every branch in branches has a data entry', () => {
    for (const branch of sampleBundle.branches) {
      assert.ok(Array.isArray(sampleBundle.data[branch]),
        `missing data for branch ${branch}`);
    }
  });

  it('branch data arrays contain valid repo objects', () => {
    for (const branch of sampleBundle.branches) {
      for (const repo of sampleBundle.data[branch]) {
        assert.ok(typeof repo.repo === 'string');
        assert.ok(typeof repo.wave === 'number');
        assert.ok(typeof repo.checks === 'object');
      }
    }
  });

  it('branch data works with transformData', () => {
    const { nodes, edges } = transformData(sampleBundle.data['oadp-1.6']);
    const repoNodes = nodes.filter(n => !n.classes);
    assert.equal(repoNodes.length, 2);
    assert.equal(edges.length, 1);
  });

  it('branch data works with classifyStatus', () => {
    const kopia = sampleBundle.data['oadp-1.6'][0];
    const velero = sampleBundle.data['oadp-1.6'][1];
    assert.equal(classifyStatus(kopia), 'done');
    assert.equal(classifyStatus(velero), 'prog');
  });
});

describe('bundle with empty branch data', () => {
  const emptyBundle = {
    branches: ['oadp-1.6'],
    generated_at: '2026-06-24T15:51:15Z',
    default_branch: 'oadp-1.6',
    data: { 'oadp-1.6': [] },
  };

  it('handles empty repo list', () => {
    const { nodes, edges } = transformData(emptyBundle.data['oadp-1.6']);
    assert.equal(nodes.length, 0);
    assert.equal(edges.length, 0);
  });
});

describe('bundle with missing branch', () => {
  const partialBundle = {
    branches: ['oadp-1.6', 'oadp-1.5'],
    generated_at: '2026-06-24T15:51:15Z',
    default_branch: 'oadp-1.6',
    data: { 'oadp-1.6': [{ repo: 'migtools/kopia', branch: 'oadp-1.6', wave: 1, skip: false, checks: {} }] },
  };

  it('gracefully handles missing branch key in data', () => {
    const data = partialBundle.data['oadp-1.5'];
    assert.equal(data, undefined);
  });
});
