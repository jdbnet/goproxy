<script setup>
import { onMounted, ref } from 'vue'
import { Trash2 } from '@lucide/vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'

const items = ref([])
const form = ref({ name: '', scopes: 'stats:read,frontends:read,backends:read,acls:read' })
const created = ref('')
const error = ref('')

async function load() {
  const { data } = await api.get('/apikeys')
  items.value = data || []
}

async function save() {
  error.value = ''
  created.value = ''
  try {
    const { data } = await api.post('/apikeys', {
      name: form.value.name,
      scopes: form.value.scopes.split(',').map((s) => s.trim()).filter(Boolean),
    })
    created.value = data.token
    form.value = { name: '', scopes: form.value.scopes }
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function askRemove(k) {
  if (!await confirm({
    title: 'Delete API key?',
    message: `Revoke "${k.name}"? Scripts using this key will stop working immediately.`,
  })) return
  await api.delete(`/apikeys/${k.id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">API Keys</h1>
      <p class="mt-1 text-sm text-muted">Tokens for scripts and automation. The secret is shown once.</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="created" class="card text-sm">Copy now, it is not shown again: <code class="break-all">{{ created }}</code></p>
    <form class="card grid gap-3 md:grid-cols-3" @submit.prevent="save">
      <input v-model="form.name" class="input-field" placeholder="Name" required />
      <input v-model="form.scopes" class="input-field" placeholder="Scopes, comma separated" required />
      <button class="btn-primary" type="submit">Create key</button>
    </form>
    <div class="card">
      <div class="table-scroll">
      <table class="data-table">
        <thead class="text-muted"><tr><th class="pb-2">Name</th><th>Prefix</th><th>Scopes</th><th></th></tr></thead>
        <tbody>
          <tr v-for="k in items" :key="k.id" class="table-row-hover">
            <td class="py-2">{{ k.name }}</td>
            <td>{{ k.prefix }}</td>
            <td>{{ (k.scopes || []).join(', ') }}</td>
            <td class="text-right">
              <button class="btn-row btn-row-danger" type="button" @click="askRemove(k)">
                <Trash2 class="h-3.5 w-3.5" />
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
  </div>
</template>
