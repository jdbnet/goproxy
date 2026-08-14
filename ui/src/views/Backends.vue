<script setup>
import { onMounted, ref } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'

import api from '@/api/client'
import { trafficLabel, trafficTitle } from '@/lib/bytes'

const items = ref([])
const stats = ref({})
const form = ref(emptyForm())
const editing = ref(false)
const error = ref('')

function emptyForm() {
  return {
    id: '',
    name: '',
    algorithm: 'round_robin',
    primary_text: 'http://127.0.0.1:3000',
    backup_text: '',
    health_type: 'none',
    health_path: '/healthz',
  }
}

function parseServers(text, role) {
  return String(text || '').split('\n').map((line) => line.trim()).filter(Boolean).map((line) => {
    const [target, weight] = line.split('|')
    const row = { role, weight: Number(weight || 1) }
    if (target.includes('://')) row.url = target.trim()
    else row.address = target.trim()
    return row
  })
}

function formatServers(servers, role) {
  return (servers || [])
    .filter((s) => (s.role || 'primary') === role)
    .map((s) => {
      const target = s.url || s.address || ''
      if (s.weight && s.weight !== 1) return `${target}|${s.weight}`
      return target
    })
    .join('\n')
}

function serverTarget(s) {
  return s?.url || s?.address || ''
}

function firstTarget(b) {
  return serverTarget(b?.servers?.[0])
}

function destinationList(b) {
  return (b?.servers || []).map(serverTarget).filter(Boolean).join(', ')
}

function healthLabel(b) {
  if (!b.health?.type) return 'Always up'
  return { http: 'HTTP', tcp: 'TCP', grpc: 'gRPC' }[b.health.type] || b.health.type
}

function roleCounts(b) {
  const servers = b?.servers || []
  const primary = servers.filter((s) => (s.role || 'primary') === 'primary').length
  const backup = servers.filter((s) => s.role === 'backup').length
  if (!backup) return `${servers.length}`
  return `${primary} primary, ${backup} backup`
}
  return {
    round_robin: 'Round robin',
    least_conn: 'Least connections',
    weighted: 'Weighted',
    ip_hash: 'Sticky by client IP',
  }[algo] || algo
}

async function load() {
  const [bes, st] = await Promise.all([
    api.get('/backends'),
    api.get('/stats').catch(() => ({ data: {} })),
  ])
  items.value = bes.data || []
  stats.value = st.data || {}
}

async function save() {
  error.value = ''
  const servers = [
    ...parseServers(form.value.primary_text, 'primary'),
    ...parseServers(form.value.backup_text, 'backup'),
  ]
  if (!servers.length) {
    error.value = 'Add at least one primary destination'
    return
  }
  const body = {
    name: form.value.name.trim(),
    algorithm: form.value.algorithm,
    servers,
  }
  if (form.value.health_type && form.value.health_type !== 'none') {
    body.health = {
      type: form.value.health_type,
      path: form.value.health_type === 'http' ? form.value.health_path : '',
      interval: '5s',
      timeout: '2s',
    }
  }
  if (editing.value) body.id = form.value.id
  try {
    if (editing.value) await api.put(`/backends/${form.value.id}`, body)
    else await api.post('/backends', body)
    reset()
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function remove(id) {
  await api.delete(`/backends/${id}`)
  await load()
}

function edit(b) {
  editing.value = true
  form.value = {
    id: b.id,
    name: b.name || '',
    algorithm: b.algorithm,
    primary_text: formatServers(b.servers, 'primary'),
    backup_text: formatServers(b.servers, 'backup'),
    health_type: b.health?.type || 'none',
    health_path: b.health?.path || '/healthz',
  }
}

function reset() {
  editing.value = false
  form.value = emptyForm()
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Backends</h1>
      <p class="mt-1 text-sm text-muted">
        Where traffic goes. Primaries take requests first. Backups are used only when every primary is down.
      </p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <form class="card grid gap-3" @submit.prevent="save">
      <div>
        <label class="mb-1 block text-sm text-muted">Name</label>
        <input v-model="form.name" class="input-field" placeholder="App servers" />
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Primary</label>
          <textarea
            v-model="form.primary_text"
            class="input-field min-h-24 font-mono"
            placeholder="http://127.0.0.1:3000"
            required
          />
          <p class="mt-1 text-xs text-muted">One URL or host:port per line. http(s):// for decrypted routes, host:port for TLS passthrough.</p>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Backup</label>
          <textarea
            v-model="form.backup_text"
            class="input-field min-h-24 font-mono"
            placeholder="http://127.0.0.1:3001"
          />
          <p class="mt-1 text-xs text-muted">Optional. Used only if all primaries fail health checks or go down.</p>
        </div>
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Balance</label>
          <select v-model="form.algorithm" class="input-field">
            <option value="round_robin">Round robin</option>
            <option value="least_conn">Least connections</option>
            <option value="weighted">Weighted</option>
            <option value="ip_hash">Sticky by client IP</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Health check</label>
          <select v-model="form.health_type" class="input-field">
            <option value="none">None (always up)</option>
            <option value="http">HTTP</option>
            <option value="tcp">TCP</option>
            <option value="grpc">gRPC</option>
          </select>
        </div>
      </div>
      <p v-if="form.health_type === 'none'" class="text-xs text-muted">
        Servers stay up even if a request fails. Use this for apps that have no health endpoint or that you never want taken out of rotation.
      </p>
      <div v-if="form.health_type === 'http'">
        <label class="mb-1 block text-sm text-muted">Health path</label>
        <input v-model="form.health_path" class="input-field" placeholder="/healthz" />
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" type="submit">{{ editing ? 'Save' : 'Add backend' }}</button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset">Cancel</button>
      </div>
    </form>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th>Destination</th>
            <th>Balance</th>
            <th>Health</th>
            <th>Servers</th>
            <th>Traffic</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="b in items" :key="b.id" class="table-row-hover">
            <td class="py-2 font-medium text-heading">{{ b.name || firstTarget(b) || b.id }}</td>
            <td class="max-w-xs truncate text-muted" :title="destinationList(b)">{{ destinationList(b) }}</td>
            <td>{{ algorithmLabel(b.algorithm) }}</td>
            <td>{{ healthLabel(b) }}</td>
            <td>{{ roleCounts(b) }}</td>
            <td class="whitespace-nowrap text-muted" :title="trafficTitle(stats.backends?.[b.id])">{{ trafficLabel(stats.backends?.[b.id]) }}</td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button class="btn-row btn-row-edit" type="button" @click="edit(b)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="remove(b.id)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="7" class="py-4 text-muted">No backends yet. Add the URL of your app.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
