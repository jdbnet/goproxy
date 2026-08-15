<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { Check, ChevronLeft, ChevronRight, LoaderCircle } from '@lucide/vue'
import SearchableSelect from '@/components/SearchableSelect.vue'
import api from '@/api/client'
import { certDisplayName, findCertificateForHost } from '@/lib/certs'
import { certID } from '@/lib/ids'

const router = useRouter()
const steps = ['Site', 'Backend', 'HTTPS', 'Listener', 'Review']
const step = ref(1)
const saving = ref(false)
const error = ref('')
const settings = ref({})

const frontends = ref([])
const backends = ref([])
const certs = ref([])
const providers = ref([])

const jobOpen = ref(false)
const job = ref({ id: '', status: 'idle', lines: [], error: '' })
let pollTimer = null

const form = ref(emptyForm())

function emptyForm() {
  return {
    domain: '',
    name: '',
    backendMode: 'new',
    backendId: '',
    backendUrl: 'http://127.0.0.1:3000',
    backendName: '',
    scheme: 'https',
    certMode: 'existing',
    certId: '',
    certChallenge: 'http-01',
    dns_provider: 'cloudflare',
    creds: {},
    cert_pem: '',
    key_pem: '',
    frontendMode: 'existing',
    frontendId: '',
    frontendBind: '0.0.0.0:443',
    frontendName: 'Public HTTPS',
    addHttpRedirect: true,
  }
}

const selectedProvider = computed(() => providers.value.find((p) => p.id === form.value.dns_provider))

const tlsFrontends = computed(() => frontends.value.filter((fe) => fe.tls?.enabled))
const plainFrontends = computed(() => frontends.value.filter((fe) => !fe.tls?.enabled))

const frontendOptions = computed(() => {
  const list = form.value.scheme === 'https' ? tlsFrontends.value : plainFrontends.value
  return list.map((fe) => ({
    value: fe.id,
    label: fe.name || fe.bind,
    hint: fe.name ? `${fe.bind} · ${fe.tls?.enabled ? 'HTTPS' : 'HTTP'}` : (fe.tls?.enabled ? 'HTTPS' : 'HTTP'),
    search: [fe.id, fe.name, fe.bind].join(' '),
  }))
})

const backendOptions = computed(() => backends.value.map((be) => {
  const target = be.servers?.[0]?.url || be.servers?.[0]?.address || ''
  const dest = (be.servers || []).map((s) => s.url || s.address).filter(Boolean).join(', ')
  return {
    value: be.id,
    label: be.name || target || be.id,
    hint: dest || be.id,
    search: [be.id, be.name, dest].join(' '),
  }
}))

const certOptions = computed(() => certs.value.map((c) => ({
  value: c.id,
  label: certDisplayName(c),
  hint: (c.domains || []).join(', '),
  search: [c.id, c.name, ...(c.domains || [])].join(' '),
})))

function pickDefaultFrontend() {
  const list = form.value.scheme === 'https' ? tlsFrontends.value : plainFrontends.value
  if (list.length === 1) {
    form.value.frontendId = list[0].id
    form.value.frontendMode = 'existing'
    return
  }
  if (form.value.scheme === 'https') {
    const on443 = list.find((fe) => fe.bind.endsWith(':443'))
    if (on443) {
      form.value.frontendId = on443.id
      form.value.frontendMode = 'existing'
    }
  }
}

function syncFromDomain() {
  const domain = form.value.domain.trim()
  if (!form.value.name) form.value.name = domain
  if (!form.value.backendName) form.value.backendName = domain
  const match = findCertificateForHost(certs.value, domain)
  if (match) {
    form.value.certMode = 'existing'
    form.value.certId = match
  }
}

watch(() => form.value.domain, syncFromDomain)
watch(() => form.value.scheme, () => {
  form.value.frontendBind = form.value.scheme === 'https' ? '0.0.0.0:443' : '0.0.0.0:80'
  form.value.frontendName = form.value.scheme === 'https' ? 'Public HTTPS' : 'HTTP'
  if (form.value.frontendMode === 'existing') {
    form.value.frontendId = ''
    pickDefaultFrontend()
  }
})

async function load() {
  const [fes, bes, cs, dns, s] = await Promise.all([
    api.get('/frontends'),
    api.get('/backends'),
    api.get('/certificates'),
    api.get('/dns-providers'),
    api.get('/settings'),
  ])
  frontends.value = fes.data || []
  backends.value = bes.data || []
  certs.value = cs.data || []
  providers.value = dns.data || []
  settings.value = s.data || {}
  if (backends.value.length) {
    form.value.backendMode = 'existing'
    if (backends.value.length === 1) form.value.backendId = backends.value[0].id
  }
  const feList = form.value.scheme === 'https' ? tlsFrontends.value : plainFrontends.value
  if (!feList.length) {
    form.value.frontendMode = 'new'
  } else {
    pickDefaultFrontend()
  }
  syncFromDomain()
}

function validateStep(n) {
  error.value = ''
  if (n === 1) {
    if (!form.value.domain.trim()) {
      error.value = 'Enter a domain'
      return false
    }
    return true
  }
  if (n === 2) {
    if (form.value.backendMode === 'existing' && !form.value.backendId) {
      error.value = 'Choose a backend'
      return false
    }
    if (form.value.backendMode === 'new' && !form.value.backendUrl.trim()) {
      error.value = 'Enter the app URL'
      return false
    }
    return true
  }
  if (n === 3) {
    if (form.value.scheme !== 'https') return true
    if (form.value.certMode === 'existing' && !form.value.certId) {
      error.value = 'Choose a certificate or request a new one'
      return false
    }
    if (form.value.certMode === 'le') {
      if (!settings.value.acme_email) {
        error.value = 'Set ACME email in Settings before requesting Let\'s Encrypt certificates'
        return false
      }
      if (form.value.certChallenge === 'dns-01') {
        const p = selectedProvider.value
        if (p?.fields?.some((f) => f.required && !String(form.value.creds[f.key] || '').trim())) {
          error.value = 'Fill in the DNS provider credentials'
          return false
        }
      }
    }
    if (form.value.certMode === 'custom' && (!form.value.cert_pem.trim() || !form.value.key_pem.trim())) {
      error.value = 'Paste certificate and private key PEMs'
      return false
    }
    return true
  }
  if (n === 4) {
    if (form.value.frontendMode === 'existing' && !form.value.frontendId) {
      error.value = 'Choose a listener'
      return false
    }
    if (form.value.frontendMode === 'new' && !form.value.frontendBind.trim()) {
      error.value = 'Enter a listen address'
      return false
    }
    return true
  }
  return true
}

function nextStep() {
  if (!validateStep(step.value)) return
  if (step.value < steps.length) step.value += 1
}

function prevStep() {
  error.value = ''
  if (step.value > 1) step.value -= 1
}

function stopPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function waitForCertJob(id) {
  jobOpen.value = true
  job.value = { id, status: 'running', lines: [], error: '' }
  return new Promise((resolve, reject) => {
    const tick = async () => {
      try {
        const { data } = await api.get(`/certificates/${id}/job`)
        job.value = data
        if (data.status === 'success') {
          stopPoll()
          resolve()
        } else if (data.status === 'error') {
          stopPoll()
          reject(new Error(data.error || 'Certificate request failed'))
        }
      } catch (e) {
        stopPoll()
        reject(e)
      }
    }
    pollTimer = setInterval(tick, 500)
    tick()
  })
}

async function ensureHttpListener() {
  const existing = frontends.value.find((fe) => fe.bind === '0.0.0.0:80' || fe.bind.endsWith(':80'))
  if (existing) return
  const body = form.value.addHttpRedirect
    ? { name: 'HTTP', bind: '0.0.0.0:80', default: { https_redirect: true } }
    : { name: 'HTTP', bind: '0.0.0.0:80' }
  await api.post('/frontends', body)
  const { data } = await api.get('/frontends')
  frontends.value = data || []
}

function validateAll() {
  for (let i = 1; i <= 4; i += 1) {
    if (!validateStep(i)) {
      step.value = i
      return false
    }
  }
  error.value = ''
  return true
}

async function createSite() {
  if (!validateAll()) return
  saving.value = true
  error.value = ''
  try {
    let backendId = form.value.backendId
    if (form.value.backendMode === 'new') {
      const { data } = await api.post('/backends', {
        name: form.value.backendName.trim() || form.value.domain.trim(),
        algorithm: 'round_robin',
        servers: [{ role: 'primary', url: form.value.backendUrl.trim(), weight: 1 }],
      })
      backendId = data.id
    }

    if (form.value.scheme === 'https' && (form.value.addHttpRedirect || (form.value.certMode === 'le' && form.value.certChallenge === 'http-01'))) {
      await ensureHttpListener()
    }

    let frontendId = form.value.frontendId
    if (form.value.frontendMode === 'new') {
      const body = {
        name: form.value.frontendName.trim() || (form.value.scheme === 'https' ? 'Public HTTPS' : 'HTTP'),
        bind: form.value.frontendBind.trim(),
      }
      if (form.value.scheme === 'https') {
        body.tls = { enabled: true, min_version: '1.2', hsts: false }
      }
      const { data } = await api.post('/frontends', body)
      frontendId = data.id
    }

    let certId = ''
    if (form.value.scheme === 'https') {
      if (form.value.certMode === 'existing') {
        certId = form.value.certId
      } else {
        const domains = [form.value.domain.trim()]
        certId = certID(domains)
        const body = {
          id: certId,
          name: form.value.name.trim() || form.value.domain.trim(),
          domains,
          challenge: form.value.certMode === 'custom' ? 'custom' : form.value.certChallenge,
          dns_provider: form.value.certChallenge === 'dns-01' ? form.value.dns_provider : '',
          dns_credentials: form.value.certChallenge === 'dns-01' ? form.value.creds : undefined,
          cert_pem: form.value.certMode === 'custom' ? form.value.cert_pem : undefined,
          key_pem: form.value.certMode === 'custom' ? form.value.key_pem : undefined,
        }
        await api.post('/certificates', body)
        if (form.value.certMode === 'le') {
          await waitForCertJob(certId)
        }
      }
    }

    await api.post('/acls', {
      name: form.value.name.trim() || form.value.domain.trim(),
      frontend: frontendId,
      match: { host: form.value.domain.trim() },
      mode: 'terminate',
      backend: backendId,
      certificate: certId,
    })

    await router.push('/routes')
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
    stopPoll()
  }
}

function readFile(ev, field) {
  const file = ev.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    form.value[field] = String(reader.result || '')
  }
  reader.readAsText(file)
}

function summaryFrontend() {
  if (form.value.frontendMode === 'new') {
    return `New listener on ${form.value.frontendBind}`
  }
  const fe = frontends.value.find((f) => f.id === form.value.frontendId)
  return fe ? `${fe.name || fe.bind} (${fe.bind})` : form.value.frontendId
}

function summaryBackend() {
  if (form.value.backendMode === 'new') return form.value.backendUrl.trim()
  const be = backends.value.find((b) => b.id === form.value.backendId)
  const t = be?.servers?.[0]?.url || be?.servers?.[0]?.address
  return be ? (be.name || t || be.id) : form.value.backendId
}

function summaryCert() {
  if (form.value.scheme !== 'https') return 'Not used (HTTP only)'
  if (form.value.certMode === 'existing') {
    const c = certs.value.find((x) => x.id === form.value.certId)
    return c ? certDisplayName(c) : form.value.certId
  }
  if (form.value.certMode === 'le') {
    return `Let's Encrypt (${form.value.certChallenge})`
  }
  return 'Custom PEM upload'
}

onMounted(load)
onUnmounted(stopPoll)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Add site</h1>
      <p class="mt-1 text-sm text-muted">
        Set up a domain, backend, certificate, and route in one go. You can reuse listeners, backends, and certs you already have.
      </p>
    </div>

    <ol class="flex flex-wrap gap-2 text-sm">
      <li
        v-for="(label, i) in steps"
        :key="label"
        class="flex items-center gap-1.5 rounded-full border px-3 py-1"
        :class="step === i + 1 ? 'border-accent bg-accent/10 text-heading' : step > i + 1 ? 'border-accent/40 text-muted' : 'border-default text-muted'"
      >
        <Check v-if="step > i + 1" class="h-3.5 w-3.5 text-accent" />
        <span v-else class="font-mono text-xs">{{ i + 1 }}</span>
        {{ label }}
      </li>
    </ol>

    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>

    <div class="card space-y-4">
      <div v-if="step === 1" class="space-y-3">
        <h2 class="font-medium text-heading">Site</h2>
        <div class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm text-muted">Domain</label>
            <input v-model="form.domain" class="input-field" placeholder="app.example.com" required />
          </div>
          <div>
            <label class="mb-1 block text-sm text-muted">Name</label>
            <input v-model="form.name" class="input-field" placeholder="App" />
          </div>
        </div>
      </div>

      <div v-else-if="step === 2" class="space-y-3">
        <h2 class="font-medium text-heading">Backend</h2>
        <div class="grid gap-2 md:grid-cols-2">
          <label class="card cursor-pointer !p-3" :class="form.backendMode === 'new' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.backendMode" type="radio" value="new" class="mr-2" />
            <span class="font-medium">New upstream</span>
            <p class="mt-1 text-xs text-muted">Point at your app URL.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.backendMode === 'existing' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.backendMode" type="radio" value="existing" class="mr-2" />
            <span class="font-medium">Existing backend</span>
            <p class="mt-1 text-xs text-muted">Reuse a pool you already defined.</p>
          </label>
        </div>
        <div v-if="form.backendMode === 'new'" class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm text-muted">App URL</label>
            <input v-model="form.backendUrl" class="input-field font-mono" placeholder="http://127.0.0.1:3000" required />
          </div>
          <div>
            <label class="mb-1 block text-sm text-muted">Backend name</label>
            <input v-model="form.backendName" class="input-field" placeholder="App servers" />
          </div>
        </div>
        <div v-else>
          <label class="mb-1 block text-sm text-muted">Backend</label>
          <SearchableSelect v-model="form.backendId" :options="backendOptions" placeholder="Choose a backend" />
        </div>
      </div>

      <div v-else-if="step === 3" class="space-y-3">
        <h2 class="font-medium text-heading">HTTPS</h2>
        <div class="grid gap-2 md:grid-cols-2">
          <label class="card cursor-pointer !p-3" :class="form.scheme === 'https' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.scheme" type="radio" value="https" class="mr-2" />
            <span class="font-medium">HTTPS</span>
            <p class="mt-1 text-xs text-muted">Terminate TLS on the listener.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.scheme === 'http' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.scheme" type="radio" value="http" class="mr-2" />
            <span class="font-medium">HTTP only</span>
            <p class="mt-1 text-xs text-muted">Plain HTTP, no certificate.</p>
          </label>
        </div>

        <template v-if="form.scheme === 'https'">
          <div class="grid gap-2 md:grid-cols-3">
            <label class="card cursor-pointer !p-3" :class="form.certMode === 'existing' ? 'ring-1 ring-accent' : ''">
              <input v-model="form.certMode" type="radio" value="existing" class="mr-2" />
              <span class="font-medium">Existing cert</span>
            </label>
            <label class="card cursor-pointer !p-3" :class="form.certMode === 'le' ? 'ring-1 ring-accent' : ''">
              <input v-model="form.certMode" type="radio" value="le" class="mr-2" />
              <span class="font-medium">Let's Encrypt</span>
            </label>
            <label class="card cursor-pointer !p-3" :class="form.certMode === 'custom' ? 'ring-1 ring-accent' : ''">
              <input v-model="form.certMode" type="radio" value="custom" class="mr-2" />
              <span class="font-medium">Upload PEM</span>
            </label>
          </div>

          <div v-if="form.certMode === 'existing'">
            <label class="mb-1 block text-sm text-muted">Certificate</label>
            <SearchableSelect v-model="form.certId" :options="certOptions" placeholder="Choose a certificate" />
            <p v-if="!certOptions.length" class="mt-1 text-xs text-muted">No certificates yet. Request one with Let's Encrypt or upload a PEM.</p>
          </div>

          <template v-else-if="form.certMode === 'le'">
            <p v-if="!settings.acme_email" class="text-sm text-amber-700 dark:text-amber-400">
              Set ACME email in <RouterLink to="/settings" class="text-accent">Settings</RouterLink> first.
            </p>
            <div class="grid gap-2 md:grid-cols-2">
              <label class="card cursor-pointer !p-3" :class="form.certChallenge === 'http-01' ? 'ring-1 ring-accent' : ''">
                <input v-model="form.certChallenge" type="radio" value="http-01" class="mr-2" />
                <span class="font-medium">HTTP-01</span>
                <p class="mt-1 text-xs text-muted">Port 80 must reach this proxy.</p>
              </label>
              <label class="card cursor-pointer !p-3" :class="form.certChallenge === 'dns-01' ? 'ring-1 ring-accent' : ''">
                <input v-model="form.certChallenge" type="radio" value="dns-01" class="mr-2" />
                <span class="font-medium">DNS-01</span>
                <p class="mt-1 text-xs text-muted">Works for wildcards. Needs DNS API access.</p>
              </label>
            </div>
            <div v-if="form.certChallenge === 'dns-01'" class="space-y-3">
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
                  <input
                    v-model="form.creds[field.key]"
                    class="input-field"
                    :type="field.secret ? 'password' : 'text'"
                    :required="field.required"
                    autocomplete="off"
                  />
                </div>
              </div>
            </div>
          </template>

          <div v-else class="grid gap-3 md:grid-cols-2">
            <div>
              <label class="mb-1 block text-sm text-muted">Certificate PEM</label>
              <input type="file" accept=".pem,.crt,.cer,.txt" class="input-field text-sm" @change="readFile($event, 'cert_pem')" />
              <textarea v-model="form.cert_pem" class="input-field mt-2 min-h-28 font-mono text-xs" placeholder="-----BEGIN CERTIFICATE-----" />
            </div>
            <div>
              <label class="mb-1 block text-sm text-muted">Private key PEM</label>
              <input type="file" accept=".pem,.key,.txt" class="input-field text-sm" @change="readFile($event, 'key_pem')" />
              <textarea v-model="form.key_pem" class="input-field mt-2 min-h-28 font-mono text-xs" placeholder="-----BEGIN PRIVATE KEY-----" />
            </div>
          </div>
        </template>
      </div>

      <div v-else-if="step === 4" class="space-y-3">
        <h2 class="font-medium text-heading">Listener</h2>
        <div class="grid gap-2 md:grid-cols-2">
          <label class="card cursor-pointer !p-3" :class="form.frontendMode === 'existing' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.frontendMode" type="radio" value="existing" class="mr-2" />
            <span class="font-medium">Existing listener</span>
            <p class="mt-1 text-xs text-muted">Reuse :443 or :80 you already run.</p>
          </label>
          <label class="card cursor-pointer !p-3" :class="form.frontendMode === 'new' ? 'ring-1 ring-accent' : ''">
            <input v-model="form.frontendMode" type="radio" value="new" class="mr-2" />
            <span class="font-medium">New listener</span>
            <p class="mt-1 text-xs text-muted">Create a bind address for this site.</p>
          </label>
        </div>
        <div v-if="form.frontendMode === 'existing'">
          <label class="mb-1 block text-sm text-muted">Listener</label>
          <SearchableSelect v-model="form.frontendId" :options="frontendOptions" placeholder="Choose a listener" />
          <p v-if="!frontendOptions.length" class="mt-1 text-xs text-muted">No matching listeners. Create a new one instead.</p>
        </div>
        <div v-else class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm text-muted">Name</label>
            <input v-model="form.frontendName" class="input-field" />
          </div>
          <div>
            <label class="mb-1 block text-sm text-muted">Listen address</label>
            <input v-model="form.frontendBind" class="input-field font-mono" placeholder="0.0.0.0:443" />
          </div>
        </div>
        <label v-if="form.scheme === 'https'" class="flex items-start gap-2 text-sm">
          <input v-model="form.addHttpRedirect" type="checkbox" class="mt-0.5" />
          <span>
            <span class="font-medium text-heading">Add HTTP listener on :80</span>
            <span class="mt-0.5 block text-muted">Creates port 80 with redirect to HTTPS if you do not have one yet. Also needed for HTTP-01 certificates.</span>
          </span>
        </label>
      </div>

      <div v-else class="space-y-3">
        <h2 class="font-medium text-heading">Review</h2>
        <dl class="grid gap-2 text-sm md:grid-cols-2">
          <div><dt class="text-muted">Domain</dt><dd class="font-medium text-heading">{{ form.domain }}</dd></div>
          <div><dt class="text-muted">Name</dt><dd class="font-medium text-heading">{{ form.name || form.domain }}</dd></div>
          <div><dt class="text-muted">Backend</dt><dd class="font-medium text-heading">{{ summaryBackend() }}</dd></div>
          <div><dt class="text-muted">HTTPS</dt><dd class="font-medium text-heading">{{ form.scheme === 'https' ? 'Yes' : 'No' }}</dd></div>
          <div><dt class="text-muted">Certificate</dt><dd class="font-medium text-heading">{{ summaryCert() }}</dd></div>
          <div><dt class="text-muted">Listener</dt><dd class="font-medium text-heading">{{ summaryFrontend() }}</dd></div>
          <div v-if="form.scheme === 'https' && form.addHttpRedirect"><dt class="text-muted">HTTP redirect</dt><dd class="font-medium text-heading">Add :80 if missing</dd></div>
        </dl>
        <p class="text-xs text-muted">Creates only the pieces you chose as new. Existing backends, certs, and listeners are linked, not copied.</p>
      </div>

      <div class="flex flex-wrap gap-2 border-t border-default pt-4">
        <button v-if="step > 1" class="btn-ghost" type="button" :disabled="saving" @click="prevStep">
          <ChevronLeft class="h-4 w-4" />
          Back
        </button>
        <button v-if="step < steps.length" class="btn-primary" type="button" @click="nextStep">
          Next
          <ChevronRight class="h-4 w-4" />
        </button>
        <button v-else class="btn-primary" type="button" :disabled="saving" @click="createSite">
          <LoaderCircle v-if="saving" class="h-4 w-4 animate-spin" />
          {{ saving ? 'Creating site…' : 'Create site' }}
        </button>
      </div>
    </div>

    <div v-if="jobOpen && saving" class="card space-y-2">
      <h2 class="text-sm font-medium text-heading">Requesting certificate</h2>
      <p class="text-xs text-muted">This can take a few minutes while Let's Encrypt validates the domain.</p>
      <pre class="max-h-48 overflow-auto rounded-lg bg-surface p-3 text-xs font-mono leading-5">{{ job.lines?.map((l) => `${l.at}  ${l.message}`).join('\n') || 'waiting…' }}</pre>
    </div>
  </div>
</template>
