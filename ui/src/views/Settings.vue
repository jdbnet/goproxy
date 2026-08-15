<script setup>
import { onMounted, ref } from 'vue'
import { Pencil, Send, Trash2 } from '@lucide/vue'
import ChangePasswordForm from '@/components/ChangePasswordForm.vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'

const TRIGGERS = [
  { id: 'backend.up', label: 'Backend up', detail: 'A server passed health checks again and is back in rotation.' },
  { id: 'backend.down', label: 'Backend down', detail: 'A server failed consecutive health checks and was taken out of rotation.' },
  { id: 'cert.renew.success', label: 'Certificate renewed', detail: 'An ACME certificate was issued or renewed.' },
  { id: 'cert.renew.failure', label: 'Certificate renewal failed', detail: 'ACME could not renew a certificate. Check HTTP-01 or DNS.' },
  { id: 'cert.expiry.warning', label: 'Certificate expiring soon', detail: 'A certificate will expire within 14 days (or your warn_before setting).' },
  { id: 'cert.expiry.expired', label: 'Certificate expired', detail: 'A certificate has passed its not-after date.' },
  { id: 'git.conflict', label: 'Git sync failed', detail: 'A pull from the Git remote failed or could not be applied.' },
  { id: 'errors.high', label: 'High error rate', detail: 'More than 20% of recent requests returned 5xx, after at least 20 requests.' },
  { id: 'backup.failure', label: 'Backup failed', detail: 'A scheduled or manual state.db snapshot failed.' },
]

const FORMATS = [
  { id: 'generic', label: 'Generic JSON', detail: 'POST {type, title, body, severity}. Use this for n8n, Gotify, or your own handler.' },
  { id: 'discord', label: 'Discord', detail: 'Sends an embed to a Discord incoming webhook URL.' },
  { id: 'slack', label: 'Slack', detail: 'Sends a text message to a Slack incoming webhook URL.' },
]

const settings = ref({})
const settingsForm = ref(emptySettingsForm())
const settingsSaving = ref(false)
const settingsError = ref('')
const settingsMsg = ref('')
const hooks = ref([])
const form = ref(emptyForm())
const editing = ref(false)
const error = ref('')
const msg = ref('')
const testing = ref('')

function emptySettingsForm() {
  return {
    acme_email: '',
    git: {
      enabled: false,
      url: '',
      branch: 'main',
      auth: 'ssh',
      key_path: '',
      token: '',
      token_set: false,
    },
  }
}

function settingsFormFrom(data) {
  const git = data.git || {}
  return {
    acme_email: data.acme_email || '',
    git: {
      enabled: git.enabled ?? data.git_enabled ?? false,
      url: git.url || data.git_url || '',
      branch: git.branch || data.git_branch || 'main',
      auth: git.auth || 'ssh',
      key_path: git.key_path || '',
      token: '',
      token_set: !!git.token_set,
    },
  }
}

function emptyForm() {
  return {
    id: '',
    url: '',
    format: 'generic',
    allEvents: true,
    triggers: [],
  }
}

function formFromHook(h) {
  const triggers = [...(h.triggers || [])]
  return {
    id: h.id || '',
    url: h.url || '',
    format: h.format || 'generic',
    allEvents: triggers.length === 0,
    triggers,
  }
}

function payloadFromForm() {
  const body = {
    url: form.value.url.trim(),
    format: form.value.format || 'generic',
    triggers: form.value.allEvents ? [] : [...form.value.triggers],
  }
  if (form.value.id) body.id = form.value.id
  return body
}

async function load() {
  const [s, h] = await Promise.all([api.get('/settings'), api.get('/notifications')])
  settings.value = s.data
  settingsForm.value = settingsFormFrom(s.data)
  hooks.value = h.data || []
}

async function saveSettings() {
  settingsError.value = ''
  settingsMsg.value = ''
  settingsSaving.value = true
  try {
    const { data } = await api.put('/settings', {
      acme_email: settingsForm.value.acme_email.trim(),
      git: {
        enabled: settingsForm.value.git.enabled,
        url: settingsForm.value.git.url.trim(),
        branch: settingsForm.value.git.branch.trim() || 'main',
        auth: settingsForm.value.git.auth,
        key_path: settingsForm.value.git.key_path.trim(),
        token: settingsForm.value.git.token.trim(),
      },
    })
    settings.value = data
    settingsForm.value = settingsFormFrom(data)
    settingsMsg.value = 'Settings saved to config.yaml'
  } catch (e) {
    settingsError.value = e.response?.data?.error || e.message
  } finally {
    settingsSaving.value = false
  }
}

async function save() {
  error.value = ''
  msg.value = ''
  const next = payloadFromForm()
  if (!next.url) {
    error.value = 'Set a webhook URL'
    return
  }
  if (!form.value.allEvents && !form.value.triggers.length) {
    error.value = 'Choose at least one trigger, or select All events'
    return
  }
  const list = editing.value
    ? hooks.value.map((h) => (h.id === next.id ? next : h))
    : [...hooks.value, next]
  try {
    const { data } = await api.put('/notifications', list)
    hooks.value = data || list
    msg.value = editing.value ? 'Webhook updated' : 'Webhook added'
    reset()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function askRemove(h) {
  const label = h.url || h.id
  if (!await confirm({
    title: 'Delete webhook?',
    message: `Remove notifications to "${label}"?`,
  })) return
  error.value = ''
  msg.value = ''
  const list = hooks.value.filter((row) => row.id !== h.id)
  try {
    const { data } = await api.put('/notifications', list)
    hooks.value = data || list
    if (form.value.id === h.id) reset()
    msg.value = 'Webhook removed'
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

function edit(h) {
  editing.value = true
  form.value = formFromHook(h)
}

function reset() {
  editing.value = false
  form.value = emptyForm()
}

function toggleTrigger(id) {
  form.value.allEvents = false
  const set = new Set(form.value.triggers)
  if (set.has(id)) set.delete(id)
  else set.add(id)
  form.value.triggers = [...set]
}

function setAllEvents() {
  form.value.allEvents = true
  form.value.triggers = []
}

function onAllEventsChange(ev) {
  if (ev.target.checked) setAllEvents()
  else form.value.allEvents = false
}

async function syncGit() {
  error.value = ''
  msg.value = ''
  try {
    const { data } = await api.post('/config/sync')
    msg.value = `Synced ${data.sha || ''}`
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function backupNow() {
  error.value = ''
  msg.value = ''
  try {
    await api.post('/backup')
    msg.value = 'Backup complete'
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function testHook(hook, key) {
  error.value = ''
  msg.value = ''
  if (!hook.url?.trim()) {
    error.value = 'Set a webhook URL first'
    return
  }
  testing.value = key
  try {
    await api.post('/notifications/test', {
      url: hook.url.trim(),
      format: hook.format || 'generic',
    })
    msg.value = 'Test sent. Look for "GoProxy test" on the other end.'
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    testing.value = ''
  }
}

function triggerLabels(h) {
  if (!h.triggers?.length) return 'All events'
  return h.triggers.map((id) => TRIGGERS.find((t) => t.id === id)?.label || id).join(', ')
}

function formatLabel(id) {
  return FORMATS.find((f) => f.id === id)?.label || 'Generic JSON'
}

function shortURL(url) {
  try {
    const u = new URL(url)
    const path = u.pathname.length > 24 ? `${u.pathname.slice(0, 20)}…` : u.pathname
    return u.host + path
  } catch {
    return url
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Settings</h1>
      <p class="mt-1 text-sm text-muted">Process settings from config.yaml, plus outbound webhooks for health and ops events.</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="msg" class="rounded-lg border border-accent/40 bg-accent/10 px-3 py-2 text-sm text-accent">{{ msg }}</p>
    <p v-if="settingsError" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ settingsError }}</p>
    <p v-if="settingsMsg" class="rounded-lg border border-accent/40 bg-accent/10 px-3 py-2 text-sm text-accent">{{ settingsMsg }}</p>

    <div class="card text-sm space-y-1">
      <div>Listen: {{ settings.listen }}</div>
      <div>Log level: {{ settings.log_level || 'info' }}</div>
      <div>Data dir: {{ settings.data_dir }}</div>
      <div>Proxy config: {{ settings.proxy_config }}</div>
      <div>Auto update: {{ settings.update?.enabled ? 'on' : 'off' }}</div>
    </div>

    <form class="card space-y-4" @submit.prevent="saveSettings">
      <div>
        <h2 class="text-sm font-medium text-heading">Let's Encrypt</h2>
        <p class="mt-1 text-sm text-muted">Required before GoProxy can request ACME certificates.</p>
      </div>
      <div>
        <label class="mb-1 block text-sm text-muted">ACME email</label>
        <input v-model="settingsForm.acme_email" class="input-field" type="email" placeholder="admin@example.com" autocomplete="email" />
      </div>

      <div>
        <h2 class="text-sm font-medium text-heading">Git sync</h2>
        <p class="mt-1 text-sm text-muted">
          Pulls proxy.yaml from a Git remote on startup and after dashboard changes.
          Commit your live config to Git before enabling sync.
        </p>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="settingsForm.git.enabled" type="checkbox" />
        <span>Enable Git sync</span>
      </label>
      <template v-if="settingsForm.git.enabled">
        <div class="grid gap-3 md:grid-cols-2">
          <div>
            <label class="mb-1 block text-sm text-muted">Repository URL</label>
            <input v-model="settingsForm.git.url" class="input-field" placeholder="git@github.com:org/goproxy-config.git" required />
          </div>
          <div>
            <label class="mb-1 block text-sm text-muted">Branch</label>
            <input v-model="settingsForm.git.branch" class="input-field" placeholder="main" />
          </div>
        </div>
        <div>
          <label class="mb-2 block text-sm text-muted">Authentication</label>
          <div class="grid gap-2 md:grid-cols-2">
            <label class="card cursor-pointer !p-3" :class="settingsForm.git.auth === 'ssh' ? 'ring-1 ring-accent' : ''">
              <input v-model="settingsForm.git.auth" type="radio" value="ssh" class="mr-2" />
              <span class="font-medium">SSH deploy key</span>
              <p class="mt-1 text-xs text-muted">Uses a private key file on this host.</p>
            </label>
            <label class="card cursor-pointer !p-3" :class="settingsForm.git.auth === 'token' ? 'ring-1 ring-accent' : ''">
              <input v-model="settingsForm.git.auth" type="radio" value="token" class="mr-2" />
              <span class="font-medium">HTTPS token</span>
              <p class="mt-1 text-xs text-muted">Personal access token or deploy token over HTTPS.</p>
            </label>
          </div>
        </div>
        <div v-if="settingsForm.git.auth === 'ssh'">
          <label class="mb-1 block text-sm text-muted">SSH key path</label>
          <input v-model="settingsForm.git.key_path" class="input-field font-mono text-xs" placeholder="/etc/goproxy/deploy_key" required />
        </div>
        <div v-else>
          <label class="mb-1 block text-sm text-muted">Git token</label>
          <input
            v-model="settingsForm.git.token"
            class="input-field"
            type="password"
            autocomplete="off"
            :placeholder="settingsForm.git.token_set ? 'leave blank to keep existing token' : 'paste token'"
            :required="!settingsForm.git.token_set"
          />
        </div>
      </template>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" type="submit" :disabled="settingsSaving">
          {{ settingsSaving ? 'Saving…' : 'Save settings' }}
        </button>
        <button class="btn-secondary" type="button" :disabled="settingsSaving" @click="syncGit">Pull Git</button>
        <button class="btn-secondary" type="button" :disabled="settingsSaving" @click="backupNow">Backup state.db</button>
      </div>
    </form>

    <ChangePasswordForm />

    <form class="card space-y-4" @submit.prevent="save">
      <div>
        <h2 class="text-sm font-medium text-heading">Webhooks</h2>
        <p class="mt-1 text-sm text-muted">
          When something important happens, GoProxy POSTs a JSON payload to a URL you choose.
          Discord and Slack wrap the same event so it shows as a normal message.
          Leave triggers empty (All events) unless you only want a subset.
        </p>
      </div>

      <div>
        <label class="mb-1 block text-sm text-muted">URL</label>
        <input v-model="form.url" class="input-field" type="url" placeholder="https://hooks.example.com/goproxy" required />
      </div>

      <div>
        <label class="mb-2 block text-sm text-muted">Format</label>
        <div class="grid gap-2 md:grid-cols-3">
          <label
            v-for="f in FORMATS"
            :key="f.id"
            class="card cursor-pointer !p-3"
            :class="form.format === f.id ? 'ring-1 ring-accent' : ''"
          >
            <input v-model="form.format" type="radio" :value="f.id" class="mr-2" />
            <span class="font-medium">{{ f.label }}</span>
            <p class="mt-1 text-xs text-muted">{{ f.detail }}</p>
          </label>
        </div>
      </div>

      <div>
        <label class="mb-2 block text-sm text-muted">When to notify</label>
        <div class="grid gap-2 md:grid-cols-2">
          <label class="card cursor-pointer !p-3" :class="form.allEvents ? 'ring-1 ring-accent' : ''">
            <input :checked="form.allEvents" type="checkbox" class="mr-2" @change="onAllEventsChange" />
            <span class="font-medium">All events</span>
            <p class="mt-1 text-xs text-muted">Every trigger below, including ones added later.</p>
          </label>
          <label
            v-for="t in TRIGGERS"
            :key="t.id"
            class="card cursor-pointer !p-3"
            :class="!form.allEvents && form.triggers.includes(t.id) ? 'ring-1 ring-accent' : ''"
          >
            <input
              :checked="form.allEvents || form.triggers.includes(t.id)"
              type="checkbox"
              class="mr-2"
              @change="toggleTrigger(t.id)"
            />
            <span class="font-medium">{{ t.label }}</span>
            <p class="mt-1 text-xs text-muted">{{ t.detail }}</p>
          </label>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" type="submit">{{ editing ? 'Save webhook' : 'Add webhook' }}</button>
        <button
          class="btn-secondary"
          type="button"
          :disabled="!!testing"
          @click="testHook(form, 'form')"
        >
          <Send class="h-3.5 w-3.5" />
          {{ testing === 'form' ? 'Sending…' : 'Send test' }}
        </button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset">Cancel</button>
      </div>
    </form>

    <div class="card">
      <div class="table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">URL</th>
            <th>Format</th>
            <th>Triggers</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="h in hooks" :key="h.id" class="table-row-hover">
            <td class="py-2 font-medium text-heading" :title="h.url">{{ shortURL(h.url) }}</td>
            <td>{{ formatLabel(h.format) }}</td>
            <td class="max-w-xs text-muted" :title="triggerLabels(h)">{{ triggerLabels(h) }}</td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button class="btn-row btn-row-renew" type="button" :disabled="!!testing" @click="testHook(h, h.id)">
                  <Send class="h-3.5 w-3.5" />
                  {{ testing === h.id ? 'Sending…' : 'Test' }}
                </button>
                <button class="btn-row btn-row-edit" type="button" @click="edit(h)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="askRemove(h)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!hooks.length">
            <td colspan="4" class="py-4 text-muted">No webhooks yet. Add a URL above to get alerts when backends or certs change.</td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
  </div>
</template>
