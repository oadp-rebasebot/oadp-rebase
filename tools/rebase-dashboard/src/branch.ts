export function branchFromSearch(search: string, branches: string[], defaultBranch: string): string {
  const requestedBranch = new URLSearchParams(search).get('branch')
  return requestedBranch && branches.includes(requestedBranch) ? requestedBranch : defaultBranch
}
