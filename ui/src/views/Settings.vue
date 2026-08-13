<script setup>
import { onMounted, ref } from 'vue'
import api from '@/api/client'

const settings = ref({})
const hooksText = ref('[]')
const error = ref('')
const msg = ref('')

async function load() {
  const [s, h] = await Promise.all([api.get('/settings'), api.get('/notifications')])
  settings.value = s.data
  hooksText.value = JSON.stringify(h.data || [], null, 2)
}

async function saveHooks() {
  error.value = ''
  msg.value = ''
  try {
    const hooks = JSON.parse(hooksText.value)
    await api.put('/notifications', hooks)
    msg.value = 'Webhooks saved'
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
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

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <h1 class="text-xl font-semibold">Settings</h1>
    <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
    <p v-if="msg" class="text-sm text-accent">{{ msg }}</p>
    <div class="card text-sm space-y-1">
      <div>Listen: {{ settings.listen }}</div>
      <div>Log level: {{ settings.log_level || 'info' }}</div>
      <div>Data dir: {{ settings.data_dir }}</div>
      <div>Proxy config: {{ settings.proxy_config }}</div>
      <div>ACME email: {{ settings.acme_email || '(not set)' }}</div>
      <div>Git: {{ settings.git_enabled ? settings.git_url : 'disabled' }}</div>
      <div>Auto update: {{ settings.update?.enabled ? 'on' : 'off' }}</div>
    </div>
    <div class="flex gap-2">
      <button class="btn-secondary" type="button" @click="syncGit">Pull Git</button>
      <button class="btn-secondary" type="button" @click="backupNow">Backup state.db</button>
    </div>
    <div class="card space-y-3">
      <h2 class="text-sm font-medium">Webhooks</h2>
      <p class="text-xs text-muted">JSON array of {url, format: generic|discord|slack, triggers}. IDs are filled in if you omit them.</p>
      <textarea v-model="hooksText" class="input-field min-h-40 font-mono text-xs" />
      <button class="btn-primary" type="button" @click="saveHooks">Save webhooks</button>
    </div>
  </div>
</template>
