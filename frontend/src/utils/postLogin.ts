const allowedPostLoginPaths = new Set(['/tasks', '/admin/users'])

export function resolvePostLoginPath(value: unknown): string {
  if (typeof value !== 'string' || value.length === 0 || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
    return '/'
  }

  try {
    const target = new URL(value, 'https://companion.invalid')
    if (target.origin !== 'https://companion.invalid' || !allowedPostLoginPaths.has(target.pathname)) return '/'
    return target.pathname + target.search + target.hash
  } catch {
    return '/'
  }
}
