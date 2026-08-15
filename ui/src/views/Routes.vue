<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'

import SearchableSelect from '@/components/SearchableSelect.vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'
import { certificateCoversHost, findCertificateForHost, hostKey } from '@/lib/certs'
import { trafficLabel, trafficTitle } from '@/lib/bytes'

const items = ref([])
const frontends = ref([])
const backends = ref([])
const certs = ref([])
const stats = ref({})
const form = ref(emptyForm())
const editing = ref(false)
const advanced = ref(false)
const error = ref('')

function emptyForm() {
  return {
    id: '',
    name: '',
    frontend: '',
    host: '',
    action: 'forward',
    mode: 'terminate',
    backend: '',
    certificate: '',
    redirect_url: '',
    redirect_keep_path: true,
    access: 'open',
    ip_allow: '',
    ip_deny: '',
    auth_realm: '',
    auth_users: '',
    auth_paths: '',
    request_headers_add: '',
    request_headers_remove: '',
    response_headers_add: '',
    response_headers_remove: '',
  }
}

function firstPick(list) {
  return list.length === 1 ? list[0].id : ''
}

function backendTarget(be) {
  const s = be?.servers?.[0]
  return s?.url || s?.address || be?.id || ''
}

function destinationList(be) {
  return (be?.servers || []).map((s) => s.url || s.address).filter(Boolean).join(', ')
}

function parseLines(text) {
  return String(text || '').split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
}

function parseUsers(text) {
  return String(text || '').split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
    const i = line.indexOf(':')
    if (i < 0) return { username: line, password: '' }
    return { username: line.slice(0, i).trim(), password: line.slice(i + 1) }
  }).filter((u) => u.username)
}

function parseHeaders(text) {
  const out = {}
  for (const line of String(text || '').split('\n')) {
    const t = line.trim()
    if (!t) continue
    const i = t.indexOf(':')
    if (i < 1) continue
    const name = t.slice(0, i).trim()
    const value = t.slice(i + 1).trim()
    if (name) out[name] = value
  }
  return out
}

function formatHeaders(map) {
  return Object.entries(map || {}).map(([k, v]) => `${k}: ${v}`).join('\n')
}

function formatUsers(auth) {
  const users = [...(auth?.users || [])]
  if (auth?.username) users.push({ username: auth.username, password: auth.password || '' })
  return users.map((u) => `${u.username}:${u.password || ''}`).join('\n')
}

function inferAction(a) {
  const m = a.middleware || {}
  if (m.redirect_url || m.domain_redirect) return 'redirect'
  if (m.https_redirect && !a.backend) return 'https'
  return 'forward'
}

function inferAccess(m) {
  const ip = !!(m.ip_allow?.length || m.ip_deny?.length)
  const auth = !!(m.basic_auth && (m.basic_auth.users?.length || m.basic_auth.username))
  if (ip && auth) return 'ip_auth'
  if (ip) return 'ip'
  if (auth) return 'auth'
  return 'open'
}

function buildMiddleware() {
  const mw = {}
  if (form.value.action === 'https') {
    mw.https_redirect = true
    return mw
  }
  if (form.value.action === 'redirect') {
    if (form.value.redirect_url.trim()) {
      mw.redirect_url = form.value.redirect_url.trim()
      mw.redirect_keep_path = form.value.redirect_keep_path
    }
    return Object.keys(mw).length ? mw : undefined
  }
  if (form.value.access === 'ip' || form.value.access === 'ip_auth') {
    const allow = parseLines(form.value.ip_allow)
    const deny = parseLines(form.value.ip_deny)
    if (allow.length) mw.ip_allow = allow
    if (deny.length) mw.ip_deny = deny
  }
  if (form.value.access === 'auth' || form.value.access === 'ip_auth') {
    const users = parseUsers(form.value.auth_users)
    if (users.length) {
      mw.basic_auth = {
        realm: form.value.auth_realm.trim(),
        users,
        paths: parseLines(form.value.auth_paths),
      }
    }
  }
  const reqAdd = parseHeaders(form.value.request_headers_add)
  const reqDel = parseLines(form.value.request_headers_remove)
  const resAdd = parseHeaders(form.value.response_headers_add)
  const resDel = parseLines(form.value.response_headers_remove)
  if (Object.keys(reqAdd).length) mw.headers_add = reqAdd
  if (reqDel.length) mw.headers_remove = reqDel
  if (Object.keys(resAdd).length) mw.response_headers_add = resAdd
  if (resDel.length) mw.response_headers_remove = resDel
  return Object.keys(mw).length ? mw : undefined
}

function forwardLabel(a) {
  if (a.middleware?.redirect_url) return `Redirect to ${a.middleware.redirect_url}`
  if (a.middleware?.domain_redirect) return `Redirect to ${a.middleware.domain_redirect}`
  if (a.middleware?.https_redirect && !a.backend) return 'Redirect to HTTPS'
  const be = backendByID(a.backend)
  return destinationList(be) || be?.name || a.backend
}

const frontendOptions = computed(() => {
  const rows = frontends.value.filter((fe) => form.value.action !== 'forward' || form.value.mode !== 'passthrough' || fe.tls?.enabled)
  return rows.map((fe) => ({
    value: fe.id,
    label: fe.name || fe.bind,
    hint: fe.name ? `${fe.bind} · ${fe.tls?.enabled ? 'HTTPS' : 'HTTP'}` : (fe.tls?.enabled ? 'HTTPS' : 'HTTP'),
    search: [fe.id, fe.name, fe.bind, fe.tls?.enabled ? 'https tls' : 'http'].join(' '),
  }))
})

const backendOptions = computed(() => backends.value.map((be) => ({
  value: be.id,
  label: be.name || backendTarget(be) || be.id,
  hint: destinationList(be) || `${(be.servers || []).length} server${(be.servers || []).length === 1 ? '' : 's'}`,
  search: [be.id, be.name, ...(be.servers || []).map((s) => s.url || s.address)].join(' '),
})))

const certOptions = computed(() => certs.value.map((c) => ({
  value: c.id,
  label: c.name || (c.domains || []).join(', ') || c.id,
  hint: c.name ? (c.domains || []).join(', ') : (c.challenge === 'custom' ? 'Custom PEM' : 'Let\'s Encrypt'),
  search: [c.id, c.name, ...(c.domains || [])].join(' '),
})))

function frontendByID(id) {
  return frontends.value.find((fe) => fe.id === id)
}

function backendByID(id) {
  return backends.value.find((be) => be.id === id)
}

function modeLabel(a) {
  if (inferAction(a) === 'redirect') return 'Redirect'
  if (inferAction(a) === 'https') return 'Force HTTPS'
  return a.mode === 'passthrough' ? 'TLS passthrough' : 'HTTPS'
}

function extras(a) {
  const out = []
  const m = a.middleware || {}
  if (m.ip_allow?.length || m.ip_deny?.length) out.push('IP lock')
  if (m.basic_auth) out.push('Auth')
  if (m.headers_add || m.headers_remove?.length || m.response_headers_add || m.response_headers_remove?.length) {
    out.push('Headers')
  }
  return out
}

function hasAdvancedFields(a) {
  if (inferAction(a) !== 'forward') return true
  return extras(a).length > 0
}

function setAdvanced(on) {
  if (!on && form.value.action !== 'forward') {
    advanced.value = true
    return
  }
  advanced.value = on
}

async function load() {
  const [acls, fes, bes, cs, st] = await Promise.all([
    api.get('/acls'),
    api.get('/frontends'),
    api.get('/backends'),
    api.get('/certificates'),
    api.get('/stats').catch(() => ({ data: {} })),
  ])
  items.value = acls.data || []
  frontends.value = fes.data || []
  backends.value = bes.data || []
  certs.value = cs.data || []
  stats.value = st.data || {}
  if (!editing.value && !form.value.frontend && !form.value.backend) {
    form.value.frontend = firstPick(frontends.value)
    form.value.backend = firstPick(backends.value)
    form.value.certificate = firstPick(certs.value)
  }
}

async function save() {
  error.value = ''
  if (!form.value.frontend) {
    error.value = 'Choose a listener'
    return
  }
  if (form.value.action === 'forward' && !form.value.backend) {
    error.value = 'Choose a backend'
    return
  }
  if (form.value.action === 'redirect' && !form.value.redirect_url.trim()) {
    error.value = 'Set a redirect URL'
    return
  }
  if ((form.value.access === 'auth' || form.value.access === 'ip_auth') && !parseUsers(form.value.auth_users).length) {
    error.value = 'Add at least one username:password'
    return
  }
  const middleware = buildMiddleware()
  const body = {
    name: form.value.name.trim(),
    frontend: form.value.frontend,
    match: { host: form.value.host.trim() },
    mode: form.value.action === 'forward' ? form.value.mode : 'terminate',
    backend: form.value.action === 'forward' ? form.value.backend : '',
    certificate: form.value.action === 'forward' && form.value.mode === 'terminate' ? form.value.certificate : '',
    middleware,
  }
  if (editing.value) body.id = form.value.id
  try {
    if (editing.value) await api.put(`/acls/${form.value.id}`, body)
    else await api.post('/acls', body)
    reset()
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function askRemove(a) {
  const label = a.name || a.match?.host || a.id
  if (!await confirm({
    title: 'Delete route?',
    message: `Remove "${label}"? Traffic matching this route will no longer be proxied.`,
  })) return
  await api.delete(`/acls/${a.id}`)
  await load()
}

function edit(a) {
  editing.value = true
  advanced.value = hasAdvancedFields(a)
  const m = a.middleware || {}
  form.value = {
    id: a.id,
    name: a.name || '',
    frontend: a.frontend,
    host: a.match?.host || '',
    action: inferAction(a),
    mode: a.mode || 'terminate',
    backend: a.backend || '',
    certificate: a.certificate || '',
    redirect_url: m.redirect_url || (m.domain_redirect ? `https://${m.domain_redirect}` : ''),
    redirect_keep_path: m.redirect_keep_path !== false,
    access: inferAccess(m),
    ip_allow: (m.ip_allow || []).join('\n'),
    ip_deny: (m.ip_deny || []).join('\n'),
    auth_realm: m.basic_auth?.realm || '',
    auth_users: formatUsers(m.basic_auth),
    auth_paths: (m.basic_auth?.paths || []).join('\n'),
    request_headers_add: formatHeaders(m.headers_add),
    request_headers_remove: (m.headers_remove || []).join('\n'),
    response_headers_add: formatHeaders(m.response_headers_add),
    response_headers_remove: (m.response_headers_remove || []).join('\n'),
  }
}

function reset() {
  editing.value = false
  advanced.value = false
  form.value = {
    ...emptyForm(),
    frontend: firstPick(frontends.value),
    backend: firstPick(backends.value),
    certificate: firstPick(certs.value),
  }
}

watch(() => form.value.mode, () => {
  const fe = frontendByID(form.value.frontend)
  if (form.value.action === 'forward' && form.value.mode === 'passthrough' && fe && !fe.tls?.enabled) {
    form.value.frontend = ''
  }
})

function syncCertificateForHost() {
  if (form.value.action !== 'forward' || form.value.mode !== 'terminate') return
  const host = form.value.host
  const match = findCertificateForHost(certs.value, host)
  if (match) {
    form.value.certificate = match
    return
  }
  if (!hostKey(host)) return
  const current = certs.value.find((c) => c.id === form.value.certificate)
  if (current && !certificateCoversHost(current, host)) {
    form.value.certificate = ''
  }
}

watch(() => form.value.host, syncCertificateForHost)
watch(() => [form.value.mode, form.value.action], syncCertificateForHost)
watch(certs, syncCertificateForHost)

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Routes</h1>
      <p class="mt-1 text-sm text-muted">
        Point a domain at a backend, pick the listener and a certificate. That covers most sites.
        Open Advanced for redirects, IP locks, passwords, and headers.
      </p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <form class="card space-y-4" @submit.prevent="save">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div class="inline-flex rounded-lg border border-default p-0.5">
          <button
            type="button"
            class="rounded-md px-3 py-1.5 text-sm"
            :class="!advanced ? 'bg-accent text-neutral-950' : 'text-muted'"
            @click="setAdvanced(false)"
          >
            Simple
          </button>
          <button
            type="button"
            class="rounded-md px-3 py-1.5 text-sm"
            :class="advanced ? 'bg-accent text-neutral-950' : 'text-muted'"
            @click="setAdvanced(true)"
          >
            Advanced
          </button>
        </div>
        <p v-if="!advanced" class="text-xs text-muted">Forwarding this domain to a backend.</p>
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Name</label>
          <input v-model="form.name" class="input-field" placeholder="App" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Domain</label>
          <input v-model="form.host" class="input-field" placeholder="app.example.com" required />
        </div>
        <div class="md:col-span-2">
          <label class="mb-1 block text-sm text-muted">Listen on</label>
          <SearchableSelect v-model="form.frontend" :options="frontendOptions" placeholder="Choose a listener" />
        </div>
      </div>

      <div v-if="advanced">
        <label class="mb-2 block text-sm text-muted">Action</label>
        <div class="grid gap-2 md:grid-cols-3">
          <label class="card cursor-pointer !p-3" :class="form.action === 'forward' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.action" type="radio" value="forward" class="mr-2" />
            <span class="font-medium">Forward</span>
            <p class="mt-1 text-xs text-muted">Send traffic to a backend. Most sites use this.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.action === 'redirect' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.action" type="radio" value="redirect" class="mr-2" />
            <span class="font-medium">Redirect</span>
            <p class="mt-1 text-xs text-muted">301 to another URL. Apex to www, old hostnames, aliases.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.action === 'https' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.action" type="radio" value="https" class="mr-2" />
            <span class="font-medium">Force HTTPS</span>
            <p class="mt-1 text-xs text-muted">HTTP listener sends browsers to the HTTPS version of this host.</p>
          </label>
        </div>
      </div>

      <div v-if="form.action === 'forward'" class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Scheme</label>
          <select v-model="form.mode" class="input-field">
            <option value="terminate">HTTPS (decrypt)</option>
            <option value="passthrough">TLS passthrough</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Forward to</label>
          <SearchableSelect v-model="form.backend" :options="backendOptions" placeholder="Choose a backend" />
        </div>
        <div v-if="form.mode === 'terminate'" class="md:col-span-2">
          <label class="mb-1 block text-sm text-muted">Certificate</label>
          <SearchableSelect
            v-model="form.certificate"
            :options="certOptions"
            placeholder="Choose a certificate"
            allow-empty
            empty-label="None (HTTP only)"
          />
        </div>
      </div>

      <div v-if="form.action === 'redirect'" class="grid gap-3 md:grid-cols-2">
        <div class="md:col-span-2">
          <label class="mb-1 block text-sm text-muted">Redirect URL</label>
          <input v-model="form.redirect_url" class="input-field" placeholder="https://www.example.com" required />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.redirect_keep_path" type="checkbox" />
          Keep path and query
        </label>
      </div>

      <p v-if="form.action === 'https'" class="text-sm text-muted">
        Use this on an HTTP listener. Browsers hitting this domain are sent to https:// with the same host and path.
      </p>

      <div v-if="advanced && form.action === 'forward'">
        <label class="mb-2 block text-sm text-muted">Who can access it</label>
        <div class="grid gap-2 md:grid-cols-4">
          <label class="card cursor-pointer !p-3" :class="form.access === 'open' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.access" type="radio" value="open" class="mr-2" />
            <span class="font-medium">Anyone</span>
            <p class="mt-1 text-xs text-muted">No extra lock.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.access === 'ip' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.access" type="radio" value="ip" class="mr-2" />
            <span class="font-medium">IP allow list</span>
            <p class="mt-1 text-xs text-muted">Only listed client IPs. Everyone else gets 403.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.access === 'auth' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.access" type="radio" value="auth" class="mr-2" />
            <span class="font-medium">Password</span>
            <p class="mt-1 text-xs text-muted">HTTP basic auth. One user or a list.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.access === 'ip_auth' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.access" type="radio" value="ip_auth" class="mr-2" />
            <span class="font-medium">IP and password</span>
            <p class="mt-1 text-xs text-muted">Must match the allow list and sign in.</p>
          </label>
        </div>
      </div>

      <div v-if="advanced && form.action === 'forward' && (form.access === 'ip' || form.access === 'ip_auth')" class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Allow only these IPs</label>
          <textarea v-model="form.ip_allow" class="input-field min-h-20 font-mono text-xs" placeholder="203.0.113.10&#10;198.51.100.0/24" />
          <p class="mt-1 text-xs text-muted">One address or CIDR per line.</p>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Also deny these IPs</label>
          <textarea v-model="form.ip_deny" class="input-field min-h-20 font-mono text-xs" placeholder="optional" />
        </div>
      </div>

      <div v-if="advanced && form.action === 'forward' && (form.access === 'auth' || form.access === 'ip_auth')" class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Realm</label>
          <input v-model="form.auth_realm" class="input-field" placeholder="Restricted" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Only these paths</label>
          <input v-model="form.auth_paths" class="input-field" placeholder="/v2" />
          <p class="mt-1 text-xs text-muted">Empty protects the whole site.</p>
        </div>
        <div class="md:col-span-2">
          <label class="mb-1 block text-sm text-muted">Users</label>
          <textarea v-model="form.auth_users" class="input-field min-h-20 font-mono text-xs" placeholder="user:password" />
          <p class="mt-1 text-xs text-muted">One username:password per line.</p>
        </div>
      </div>

      <div v-if="advanced && form.action === 'forward'" class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Add request headers</label>
          <textarea v-model="form.request_headers_add" class="input-field min-h-20 font-mono text-xs" placeholder="X-Example: value" />
          <p class="mt-1 text-xs text-muted">One Name: value per line. Sent to the backend. X-Forwarded-Proto, Host, For, and Port are set for you.</p>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Remove request headers</label>
          <textarea v-model="form.request_headers_remove" class="input-field min-h-20 font-mono text-xs" placeholder="Cookie" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Add response headers</label>
          <textarea v-model="form.response_headers_add" class="input-field min-h-20 font-mono text-xs" placeholder="Content-Security-Policy: frame-ancestors 'self' https://dashboard.example.com" />
          <p class="mt-1 text-xs text-muted">Overwrites the same header from the backend. Use this for CSP or embedding.</p>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Remove response headers</label>
          <textarea v-model="form.response_headers_remove" class="input-field min-h-20 font-mono text-xs" placeholder="X-Frame-Options" />
        </div>
      </div>

      <div class="flex gap-2">
        <button class="btn-primary" type="submit">{{ editing ? 'Save' : 'Add route' }}</button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset">Cancel</button>
      </div>
    </form>
    <div class="card">
      <div class="table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th>Domain</th>
            <th>Action</th>
            <th>Listen</th>
            <th>Forward</th>
            <th>Traffic</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in items" :key="a.id" class="table-row-hover">
            <td class="py-2">
              <div class="font-medium text-heading">{{ a.name || a.match?.host }}</div>
              <div v-if="extras(a).length" class="mt-1 flex flex-wrap gap-1">
                <span v-for="x in extras(a)" :key="x" class="badge badge-stopped">{{ x }}</span>
              </div>
            </td>
            <td class="text-muted">{{ a.match?.host }}</td>
            <td>{{ modeLabel(a) }}</td>
            <td>{{ frontendByID(a.frontend)?.name || frontendByID(a.frontend)?.bind || a.frontend }}</td>
            <td class="text-muted" :title="forwardLabel(a)">{{ forwardLabel(a) }}</td>
            <td class="whitespace-nowrap text-muted" :title="trafficTitle(stats.routes?.[a.id])">{{ trafficLabel(stats.routes?.[a.id]) }}</td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button class="btn-row btn-row-edit" type="button" @click="edit(a)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="askRemove(a)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="7" class="py-4 text-muted">No routes yet. Add a domain, listener, backend, and certificate.</td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
  </div>
</template>
