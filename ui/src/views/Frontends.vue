<script setup>
import { computed, onMounted, ref } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'

import SearchableSelect from '@/components/SearchableSelect.vue'
import api from '@/api/client'
import { trafficLabel, trafficTitle } from '@/lib/bytes'

const items = ref([])
const backends = ref([])
const stats = ref({})
const form = ref(emptyForm())
const editing = ref(false)
const error = ref('')

function emptyForm() {
  return {
    id: '',
    name: '',
    bind: '0.0.0.0:443',
    tls: { enabled: true, min_version: '1.2', hsts: false },
    fallback: 'none',
    redirect_url: '',
    redirect_keep_path: true,
    backend: '',
  }
}

function firstTarget(be) {
  const s = be?.servers?.[0]
  return s?.url || s?.address || be?.id || ''
}

function destinationList(be) {
  return (be?.servers || []).map((s) => s.url || s.address).filter(Boolean).join(', ')
}

const backendOptions = computed(() => backends.value.map((be) => ({
  value: be.id,
  label: be.name || firstTarget(be) || be.id,
  hint: destinationList(be),
  search: [be.id, be.name, destinationList(be)].join(' '),
})))

function backendByID(id) {
  return backends.value.find((be) => be.id === id)
}

function inferFallback(fe) {
  if (fe.default?.redirect_url) return 'redirect'
  if (fe.default?.backend) return 'forward'
  return 'none'
}

function defaultLabel(fe) {
  if (fe.default?.redirect_url) return `Redirect to ${fe.default.redirect_url}`
  if (fe.default?.backend) {
    const be = backendByID(fe.default.backend)
    return be?.name || firstTarget(be) || fe.default.backend
  }
  return 'None'
}

async function load() {
  const [fes, bes, st] = await Promise.all([
    api.get('/frontends'),
    api.get('/backends'),
    api.get('/stats').catch(() => ({ data: {} })),
  ])
  items.value = fes.data || []
  backends.value = bes.data || []
  stats.value = st.data || {}
}

async function save() {
  error.value = ''
  if (form.value.fallback === 'redirect' && !form.value.redirect_url.trim()) {
    error.value = 'Set a redirect URL'
    return
  }
  if (form.value.fallback === 'forward' && !form.value.backend) {
    error.value = 'Choose a default backend'
    return
  }
  try {
    const body = { name: form.value.name.trim(), bind: form.value.bind }
    if (form.value.tls.enabled) {
      body.tls = { enabled: true, min_version: form.value.tls.min_version || '1.2', hsts: form.value.tls.hsts }
    }
    if (form.value.fallback === 'redirect') {
      body.default = {
        redirect_url: form.value.redirect_url.trim(),
        redirect_keep_path: form.value.redirect_keep_path,
      }
    } else if (form.value.fallback === 'forward') {
      body.default = { backend: form.value.backend }
    }
    if (editing.value) await api.put(`/frontends/${form.value.id}`, { ...body, id: form.value.id })
    else await api.post('/frontends', body)
    reset()
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function remove(id) {
  await api.delete(`/frontends/${id}`)
  await load()
}

function edit(fe) {
  editing.value = true
  form.value = {
    id: fe.id,
    name: fe.name || '',
    bind: fe.bind,
    tls: fe.tls || { enabled: false, min_version: '1.2', hsts: false },
    fallback: inferFallback(fe),
    redirect_url: fe.default?.redirect_url || '',
    redirect_keep_path: fe.default?.redirect_keep_path !== false,
    backend: fe.default?.backend || '',
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
      <h1 class="text-xl font-semibold">Frontends</h1>
      <p class="mt-1 text-sm text-muted">
        Listeners that accept traffic. A default is used when no route matches the host.
      </p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <form class="card space-y-4" @submit.prevent="save">
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Name</label>
          <input v-model="form.name" class="input-field" placeholder="Public HTTPS" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Listen address</label>
          <input v-model="form.bind" class="input-field" placeholder="0.0.0.0:443" required />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.tls.enabled" type="checkbox" />
          HTTPS / TLS
        </label>
        <label v-if="form.tls.enabled" class="flex items-center gap-2 text-sm">
          <input v-model="form.tls.hsts" type="checkbox" />
          Send HSTS header
        </label>
      </div>

      <div>
        <label class="mb-2 block text-sm text-muted">When nothing matches</label>
        <div class="grid gap-2 md:grid-cols-3">
          <label class="card cursor-pointer !p-3" :class="form.fallback === 'none' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.fallback" type="radio" value="none" class="mr-2" />
            <span class="font-medium">None</span>
            <p class="mt-1 text-xs text-muted">Unknown hosts get 404 or the connection is closed.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.fallback === 'redirect' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.fallback" type="radio" value="redirect" class="mr-2" />
            <span class="font-medium">Redirect</span>
            <p class="mt-1 text-xs text-muted">Send unmatched hosts to a URL, with or without the path.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.fallback === 'forward' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.fallback" type="radio" value="forward" class="mr-2" />
            <span class="font-medium">Forward</span>
            <p class="mt-1 text-xs text-muted">Send unmatched hosts to a backend.</p>
          </label>
        </div>
      </div>

      <div v-if="form.fallback === 'redirect'" class="grid gap-3 md:grid-cols-2">
        <div class="md:col-span-2">
          <label class="mb-1 block text-sm text-muted">Redirect URL</label>
          <input v-model="form.redirect_url" class="input-field" placeholder="https://www.example.com" required />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.redirect_keep_path" type="checkbox" />
          Keep path and query
        </label>
      </div>

      <div v-if="form.fallback === 'forward'">
        <label class="mb-1 block text-sm text-muted">Default backend</label>
        <SearchableSelect v-model="form.backend" :options="backendOptions" placeholder="Choose a backend" />
      </div>

      <div class="flex gap-2">
        <button class="btn-primary" type="submit">{{ editing ? 'Save' : 'Add listener' }}</button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset">Cancel</button>
      </div>
    </form>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th>Address</th>
            <th>Protocol</th>
            <th>Default</th>
            <th>Traffic</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="fe in items" :key="fe.id" class="table-row-hover">
            <td class="py-2 font-medium text-heading">{{ fe.name || fe.bind }}</td>
            <td class="text-muted">{{ fe.bind }}</td>
            <td>
              <span :class="fe.tls?.enabled ? 'badge badge-running' : 'badge badge-stopped'">
                {{ fe.tls?.enabled ? 'HTTPS' : 'HTTP' }}
              </span>
              <span v-if="fe.tls?.hsts" class="ml-2 text-xs text-muted">HSTS</span>
            </td>
            <td class="max-w-xs truncate text-muted" :title="defaultLabel(fe)">{{ defaultLabel(fe) }}</td>
            <td class="whitespace-nowrap text-muted" :title="trafficTitle(stats.frontends?.[fe.id])">{{ trafficLabel(stats.frontends?.[fe.id]) }}</td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button class="btn-row btn-row-edit" type="button" @click="edit(fe)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="remove(fe.id)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="6" class="py-4 text-muted">No listeners yet. Add 0.0.0.0:80 and 0.0.0.0:443 to start.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
