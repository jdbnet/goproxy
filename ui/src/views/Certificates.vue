<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { FileUp, LoaderCircle, Pencil, RefreshCw, Trash2, X } from '@lucide/vue'
import { certID } from '@/lib/ids'
import api from '@/api/client'

const items = ref([])
const providers = ref([])
const error = ref('')
const msg = ref('')
const editing = ref(false)
const saving = ref(false)
const jobOpen = ref(false)
const job = ref({ id: '', status: 'idle', action: '', lines: [], error: '' })
const logEl = ref(null)
let pollTimer = null

const emptyForm = () => ({
  id: '',
  name: '',
  domains: '',
  challenge: 'http-01',
  dns_provider: 'cloudflare',
  creds: {},
  cert_pem: '',
  key_pem: '',
})

const form = ref(emptyForm())
const certFileName = ref('')
const keyFileName = ref('')

const selectedProvider = computed(() => providers.value.find((p) => p.id === form.value.dns_provider))

async function load() {
  const [certs, dns] = await Promise.all([
    api.get('/certificates'),
    api.get('/dns-providers'),
  ])
  items.value = certs.data || []
  providers.value = dns.data || []
}

function reset() {
  editing.value = false
  form.value = emptyForm()
  certFileName.value = ''
  keyFileName.value = ''
}

function edit(c) {
  editing.value = true
  form.value = {
    id: c.id,
    name: c.name || '',
    domains: (c.domains || []).join(', '),
    challenge: c.challenge,
    dns_provider: c.dns_provider || 'cloudflare',
    creds: {},
    cert_pem: '',
    key_pem: '',
  }
  certFileName.value = ''
  keyFileName.value = ''
}

watch(() => form.value.dns_provider, () => {
  if (!editing.value) {
    form.value.creds = {}
  }
})

function readFile(ev, field) {
  const file = ev.target.files?.[0]
  if (!file) return
  if (field === 'cert_pem') certFileName.value = file.name
  if (field === 'key_pem') keyFileName.value = file.name
  const reader = new FileReader()
  reader.onload = () => {
    form.value[field] = String(reader.result || '')
  }
  reader.readAsText(file)
}

async function refreshJob(id) {
  try {
    const { data } = await api.get(`/certificates/${id}/job`)
    job.value = data
    await nextTick()
    if (logEl.value) logEl.value.scrollTop = logEl.value.scrollHeight
  } catch {
    // keep last job snapshot
  }
}

function startPoll(id) {
  stopPoll()
  jobOpen.value = true
  job.value = { id, status: 'running', action: 'issue', lines: [], error: '' }
  pollTimer = setInterval(() => refreshJob(id), 500)
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function withJob(id, fn) {
  error.value = ''
  msg.value = ''
  saving.value = true
  startPoll(id)
  try {
    await fn()
    await refreshJob(id)
    if (job.value.status !== 'error') {
      msg.value = job.value.action === 'renew' ? `Renewed ${id}` : 'Certificate saved'
      reset()
      await load()
    } else {
      error.value = job.value.error || 'Certificate job failed'
    }
  } catch (e) {
    await refreshJob(id)
    error.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
    stopPoll()
    await refreshJob(id)
  }
}

async function save() {
  const domains = form.value.domains.split(/[,\s]+/).map((s) => s.trim()).filter(Boolean)
  const id = editing.value ? form.value.id : certID(domains)
  const body = {
    id,
    name: form.value.name.trim(),
    domains,
    challenge: form.value.challenge,
    dns_provider: form.value.challenge === 'dns-01' ? form.value.dns_provider : '',
    dns_credentials: form.value.challenge === 'dns-01' ? form.value.creds : undefined,
    cert_pem: form.value.challenge === 'custom' ? form.value.cert_pem : undefined,
    key_pem: form.value.challenge === 'custom' ? form.value.key_pem : undefined,
  }
  await withJob(id, async () => {
    if (editing.value) await api.put(`/certificates/${id}`, body)
    else await api.post('/certificates', body)
  })
}

async function renew(id) {
  await withJob(id, () => api.post(`/certificates/${id}/renew`))
}

async function remove(id) {
  await api.delete(`/certificates/${id}`)
  await load()
}

onMounted(load)
onUnmounted(stopPoll)

const jobRunning = computed(() => saving.value || job.value.status === 'running')

function formatExpiry(iso, daysLeft) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const when = d.toISOString().replace('T', ' ').replace(/\.\d+Z$/, ' UTC')
  if (daysLeft == null) return when
  if (daysLeft < 0) return `${when} (expired)`
  if (daysLeft === 0) return `${when} (today)`
  if (daysLeft === 1) return `${when} (1 day)`
  return `${when} (${daysLeft} days)`
}

function expiryClass(daysLeft) {
  if (daysLeft == null) return ''
  if (daysLeft < 0) return 'text-red-600 dark:text-red-400'
  if (daysLeft <= 14) return 'text-amber-700 dark:text-amber-400'
  return ''
}
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Certificates</h1>
      <p class="mt-1 text-sm text-muted">
        Issue a Let's Encrypt certificate or upload your own PEM files.
        DNS tokens stay on this host, not in Git.
        See <RouterLink to="/help" class="text-accent">Getting started</RouterLink>.
      </p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="msg" class="text-sm text-accent">{{ msg }}</p>

    <form class="card space-y-4" @submit.prevent="save">
      <div class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Name</label>
          <input v-model="form.name" class="input-field" placeholder="App cert" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Domains</label>
          <input v-model="form.domains" class="input-field" placeholder="app.example.com, www.example.com" required />
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm text-muted">Challenge</label>
        <div class="grid gap-2 md:grid-cols-3">
          <label class="card cursor-pointer !p-3" :class="form.challenge === 'http-01' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.challenge" type="radio" value="http-01" class="mr-2" />
            <span class="font-medium">HTTP-01</span>
            <p class="mt-1 text-xs text-muted">Needs a frontend on port 80. GoProxy serves the ACME challenge path.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.challenge === 'dns-01' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.challenge" type="radio" value="dns-01" class="mr-2" />
            <span class="font-medium">DNS-01</span>
            <p class="mt-1 text-xs text-muted">Works for wildcards. Uses your DNS provider API.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.challenge === 'custom' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.challenge" type="radio" value="custom" class="mr-2" />
            <span class="font-medium">Custom PEM</span>
            <p class="mt-1 text-xs text-muted">Upload a certificate and private key you already have.</p>
          </label>
        </div>
      </div>

      <div v-if="form.challenge === 'dns-01'" class="space-y-3">
        <div>
          <label class="mb-1 block text-sm text-muted">DNS provider</label>
          <select v-model="form.dns_provider" class="input-field">
            <option v-for="p in providers" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div v-if="selectedProvider" class="grid gap-3 md:grid-cols-2">
          <div v-for="field in selectedProvider.fields" :key="field.key">
            <label class="mb-1 block text-sm text-muted">
              {{ field.label }}
              <span v-if="field.required" class="text-red-500">*</span>
            </label>
            <textarea
              v-if="field.key === 'EXTRA_ENV'"
              v-model="form.creds[field.key]"
              class="input-field min-h-24 font-mono text-xs"
              :placeholder="field.help || 'KEY=value'"
            />
            <input
              v-else
              v-model="form.creds[field.key]"
              class="input-field"
              :type="field.secret ? 'password' : 'text'"
              :placeholder="field.placeholder || (editing ? 'leave blank to keep existing' : '')"
              :required="field.required && !editing"
              autocomplete="off"
            />
            <p v-if="field.help" class="mt-1 text-xs text-muted">{{ field.help }}</p>
          </div>
        </div>
      </div>

      <div v-if="form.challenge === 'custom'" class="grid gap-3 md:grid-cols-2">
        <div>
          <label class="mb-1 block text-sm text-muted">Certificate PEM</label>
          <label class="btn-secondary mb-2 w-full cursor-pointer">
            <input type="file" accept=".pem,.crt,.cer,.txt" class="sr-only" @change="readFile($event, 'cert_pem')" />
            <FileUp class="h-4 w-4 shrink-0" />
            <span class="truncate">{{ certFileName || 'Choose file' }}</span>
          </label>
          <textarea v-model="form.cert_pem" class="input-field min-h-32 font-mono text-xs" placeholder="-----BEGIN CERTIFICATE-----" :required="!editing" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-muted">Private key PEM</label>
          <label class="btn-secondary mb-2 w-full cursor-pointer">
            <input type="file" accept=".pem,.key,.txt" class="sr-only" @change="readFile($event, 'key_pem')" />
            <FileUp class="h-4 w-4 shrink-0" />
            <span class="truncate">{{ keyFileName || 'Choose file' }}</span>
          </label>
          <textarea v-model="form.key_pem" class="input-field min-h-32 font-mono text-xs" placeholder="-----BEGIN PRIVATE KEY-----" :required="!editing" />
        </div>
      </div>

      <div class="flex gap-2">
        <button class="btn-primary" type="submit" :disabled="saving">
          <LoaderCircle v-if="saving" class="mr-2 h-4 w-4 animate-spin" />
          {{ editing ? 'Update' : 'Save' }}
        </button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset" :disabled="saving">Cancel</button>
      </div>
    </form>

    <div v-if="jobOpen" class="card space-y-3">
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <LoaderCircle v-if="jobRunning" class="h-5 w-5 animate-spin text-accent" />
          <h2 class="font-medium">
            {{ jobRunning ? 'Working on certificate' : job.status === 'error' ? 'Certificate job failed' : 'Certificate job finished' }}
            <span class="text-sm font-normal text-muted">{{ job.id }}</span>
          </h2>
        </div>
        <button class="btn-ghost px-2" type="button" :disabled="jobRunning" @click="jobOpen = false">
          <X class="h-4 w-4" />
        </button>
      </div>
      <p class="text-xs text-muted">
        HTTP-01 and DNS-01 can take a few minutes while Let's Encrypt validates the challenge.
        DNS-01 waits for record propagation.
      </p>
      <pre
        ref="logEl"
        class="max-h-72 overflow-auto rounded-lg bg-surface p-3 text-xs font-mono leading-5 text-heading"
      ><template v-for="(line, i) in job.lines" :key="i">{{ line.at }}  {{ line.message }}
</template><template v-if="!job.lines?.length && jobRunning">waiting for log output...
</template></pre>
    </div>

    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Name</th>
            <th>Domains</th>
            <th>Method</th>
            <th>Expires</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in items" :key="c.id" class="table-row-hover">
            <td class="py-2 font-medium text-heading">{{ c.name || (c.domains || []).join(', ') }}</td>
            <td class="text-muted">{{ (c.domains || []).join(', ') }}</td>
            <td>{{ c.challenge === 'custom' ? 'Custom PEM' : c.challenge === 'dns-01' ? `Let's Encrypt (DNS${c.dns_provider ? `: ${c.dns_provider}` : ''})` : 'Let\'s Encrypt (HTTP)' }}</td>
            <td>
              <span v-if="c.expires_at" :class="expiryClass(c.days_left)">{{ formatExpiry(c.expires_at, c.days_left) }}</span>
              <span v-else class="text-muted">not issued</span>
            </td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button v-if="c.challenge !== 'custom'" class="btn-row btn-row-renew" type="button" :disabled="saving" @click="renew(c.id)">
                  <RefreshCw class="h-3.5 w-3.5" />
                  Renew
                </button>
                <button class="btn-row btn-row-edit" type="button" @click="edit(c)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="remove(c.id)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="5" class="py-4 text-muted">No certificates yet. Add a domain and pick a challenge.</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
