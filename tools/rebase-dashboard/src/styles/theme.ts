export const colors = {
  bg: '#0d1117',
  surface: '#161b22',
  surface2: '#1c2129',
  border: '#30363d',
  text: '#c9d1d9',
  muted: '#8b949e',
  red: '#f85149',
  yellow: '#d29922',
  green: '#3fb950',
  blue: '#58a6ff',
  cyan: '#79c0ff',
  purple: '#bc8cff',
} as const

export const statusColors: Record<string, { bg: string; border: string; text: string }> = {
  done: { bg: '#238636', border: '#3fb950', text: '#ffffff' },
  need: { bg: '#9a6700', border: '#d29922', text: '#ffffff' },
  prog: { bg: '#1f6feb', border: '#58a6ff', text: '#ffffff' },
  skip: { bg: '#30363d', border: '#8b949e', text: '#8b949e' },
}

export const orgColors: Record<string, string> = {
  openshift: '#58a6ff',
  migtools: '#bc8cff',
}
