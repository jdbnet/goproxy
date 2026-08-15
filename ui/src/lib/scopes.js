export const SCOPE_RESOURCES = [
  { id: 'frontends', label: 'Frontends' },
  { id: 'backends', label: 'Backends' },
  { id: 'acls', label: 'Routes' },
  { id: 'certs', label: 'Certificates' },
  { id: 'users', label: 'Users' },
  { id: 'apikeys', label: 'API keys' },
  { id: 'audit', label: 'Audit log' },
  { id: 'stats', label: 'Stats' },
  { id: 'settings', label: 'Settings' },
]

export const SCOPE_ACTIONS = [
  { id: 'read', label: 'Read' },
  { id: 'write', label: 'Write' },
]

export function scopeID(resource, action) {
  return `${resource}:${action}`
}

export function allScopeIDs() {
  return SCOPE_RESOURCES.flatMap((r) => SCOPE_ACTIONS.map((a) => scopeID(r.id, a.id)))
}

export const DEFAULT_API_KEY_SCOPES = [
  'stats:read',
  'frontends:read',
  'backends:read',
  'acls:read',
]
