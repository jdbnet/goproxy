export function slug(s) {
  const raw = String(s || '').toLowerCase().trim()
  let out = ''
  let prevDash = false
  for (const ch of raw) {
    if (/[a-z0-9]/.test(ch)) {
      out += ch
      prevDash = false
    } else if (!prevDash && out.length) {
      out += '-'
      prevDash = true
    }
  }
  out = out.replace(/^-+|-+$/g, '')
  return out || 'item'
}

export function certID(domains) {
  const first = String(domains?.[0] || '').trim()
  if (first.startsWith('*.')) return `wildcard-${slug(first.slice(2))}`
  return slug(first) || 'cert'
}
