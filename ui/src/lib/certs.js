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
