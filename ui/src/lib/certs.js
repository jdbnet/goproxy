export function hostKey(host) {
  const h = String(host || '').trim().toLowerCase()
  const i = h.indexOf(':')
  return i >= 0 ? h.slice(0, i) : h
}

export function certMatchesHost(pattern, host) {
  const p = String(pattern || '').trim().toLowerCase()
  const h = hostKey(host)
  if (!p || !h) return false
  if (p.startsWith('*.')) {
    const suffix = p.slice(1)
    return h.length > suffix.length && h.endsWith(suffix) && h !== suffix.slice(1)
  }
  return p === h
}

export function findCertificateForHost(certs, host) {
  const h = hostKey(host)
  if (!h) return ''
  let wildcardId = ''
  let wildcardSuffixLen = 0
  for (const cert of certs || []) {
    for (const domain of cert.domains || []) {
      const p = String(domain || '').trim().toLowerCase()
      if (!p) continue
      if (p === h) return cert.id
      if (p.startsWith('*.')) {
        const suffix = p.slice(1)
        if (h.length > suffix.length && h.endsWith(suffix) && h !== suffix.slice(1) && suffix.length > wildcardSuffixLen) {
          wildcardId = cert.id
          wildcardSuffixLen = suffix.length
        }
      }
    }
  }
  return wildcardId
}

export function certificateCoversHost(cert, host) {
  if (!cert) return false
  return (cert.domains || []).some((d) => certMatchesHost(d, host))
}

export const DEFAULT_EXPIRY_WARN_DAYS = 14

export function expiryTier(daysLeft) {
  if (daysLeft == null) return 'unknown'
  if (daysLeft < 0) return 'expired'
  if (daysLeft <= DEFAULT_EXPIRY_WARN_DAYS) return 'warning'
  return 'ok'
}

export function expiryBadge(daysLeft) {
  const tier = expiryTier(daysLeft)
  if (tier === 'expired') return { label: 'Expired', class: 'badge-error' }
  if (tier === 'warning') {
    const label = daysLeft === 0 ? 'Expires today' : daysLeft === 1 ? '1 day left' : `${daysLeft} days left`
    return { label, class: daysLeft <= 3 ? 'badge-error' : 'badge-warn' }
  }
  return null
}

export function certDisplayName(cert) {
  if (!cert) return ''
  if (cert.name) return cert.name
  const domains = cert.domains || []
  if (domains.length) return domains.join(', ')
  return cert.id || ''
}

export function filterExpiringCerts(certs, warnDays = DEFAULT_EXPIRY_WARN_DAYS) {
  return (certs || []).filter((c) => c.days_left != null && c.days_left <= warnDays)
}

export function sortByExpiry(certs) {
  return [...(certs || [])].sort((a, b) => {
    const ad = a.days_left ?? 999999
    const bd = b.days_left ?? 999999
    if (ad !== bd) return ad - bd
    return certDisplayName(a).localeCompare(certDisplayName(b))
  })
}
