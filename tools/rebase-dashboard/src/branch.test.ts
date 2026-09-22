import { describe, expect, it } from 'vitest'
import { branchFromSearch } from './branch'

describe('branchFromSearch', () => {
  const branches = ['oadp-1.6', 'oadp-1.5']

  it('uses a valid branch query parameter', () => {
    expect(branchFromSearch('?branch=oadp-1.5', branches, 'oadp-1.6')).toBe('oadp-1.5')
  })

  it('falls back to the default branch for an unknown or missing parameter', () => {
    expect(branchFromSearch('?branch=oadp-9.9', branches, 'oadp-1.6')).toBe('oadp-1.6')
    expect(branchFromSearch('', branches, 'oadp-1.6')).toBe('oadp-1.6')
  })
})
