export function formatBytes(n) {
  const v = Number(n) || 0
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let x = v
  let i = 0
  while (x >= 1024 && i < units.length - 1) {
    x /= 1024
    i++
  }
  if (i === 0) return `${Math.round(x)} B`
  return `${x.toFixed(x >= 10 ? 1 : 2)} ${units[i]}`
}

export function trafficLabel(s) {
  if (!s) return '0 B'
  return formatBytes((Number(s.bytes_in) || 0) + (Number(s.bytes_out) || 0))
}

export function trafficTitle(s) {
  if (!s) return 'No traffic yet'
  return `In ${formatBytes(s.bytes_in)} · Out ${formatBytes(s.bytes_out)} · ${s.requests || 0} requests`
}

export function formatLatency(ms) {
  const v = Number(ms)
  if (!Number.isFinite(v) || v <= 0) return '—'
  if (v < 1) return `${v.toFixed(2)} ms`
  if (v < 10) return `${v.toFixed(1)} ms`
  if (v < 1000) return `${Math.round(v)} ms`
  return `${(v / 1000).toFixed(2)} s`
}

export function formatUptime(sec) {
  const s = Math.max(0, Math.floor(Number(sec) || 0))
  const d = Math.floor(s / 86400)
  const h = Math.floor((s % 86400) / 3600)
  const m = Math.floor((s % 3600) / 60)
  const r = s % 60
  if (d > 0) return `${d}d ${h}h`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${r}s`
  return `${r}s`
}
