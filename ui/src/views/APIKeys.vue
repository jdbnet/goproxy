<script setup>
import { onMounted, ref } from 'vue'
import { Pencil, Trash2 } from '@lucide/vue'
import api from '@/api/client'
import { confirm } from '@/lib/confirm'
import {
  SCOPE_RESOURCES,
  SCOPE_ACTIONS,
  scopeID,
  DEFAULT_API_KEY_SCOPES,
} from '@/lib/scopes'

const items = ref([])
const editing = ref(false)
const form = ref({ id: null, name: '', scopes: [...DEFAULT_API_KEY_SCOPES] })
const created = ref('')
const error = ref('')

function reset() {
  editing.value = false
  form.value = { id: null, name: '', scopes: [...DEFAULT_API_KEY_SCOPES] }
}

function hasScope(scope) {
  return form.value.scopes.includes(scope)
}

function toggleScope(scope) {
  const scopes = new Set(form.value.scopes)
  if (scopes.has(scope)) scopes.delete(scope)
  else scopes.add(scope)
  form.value.scopes = [...scopes]
}

function toggleResource(resource, action) {
  toggleScope(scopeID(resource, action))
}

async function load() {
  const { data } = await api.get('/apikeys')
  items.value = data || []
}

async function save() {
  error.value = ''
  created.value = ''
  if (!form.value.scopes.length) {
    error.value = 'Select at least one scope'
    return
  }
  try {
    if (editing.value) {
      await api.put(`/apikeys/${form.value.id}`, { scopes: form.value.scopes })
      reset()
    } else {
      const { data } = await api.post('/apikeys', {
        name: form.value.name,
        scopes: form.value.scopes,
      })
      created.value = data.token
      form.value = { id: null, name: '', scopes: [...form.value.scopes] }
    }
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

function edit(k) {
  editing.value = true
  created.value = ''
  error.value = ''
  form.value = {
    id: k.id,
    name: k.name,
    scopes: [...(k.scopes || [])],
  }
}

async function askRemove(k) {
  if (!await confirm({
    title: 'Delete API key?',
    message: `Revoke "${k.name}"? Scripts using this key will stop working immediately.`,
  })) return
  await api.delete(`/apikeys/${k.id}`)
  if (editing.value && form.value.id === k.id) reset()
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
    <form class="card space-y-4" @submit.prevent="save">
      <div>
        <label class="mb-1 block text-sm text-muted">Name</label>
        <input
          v-model="form.name"
          class="input-field max-w-md"
          placeholder="Name"
          :disabled="editing"
          required
        />
      </div>
      <div>
        <label class="mb-2 block text-sm text-muted">Permissions</label>
        <div class="table-scroll">
          <table class="data-table text-sm">
            <thead class="text-muted">
              <tr>
                <th class="pb-2 text-left">Resource</th>
                <th v-for="action in SCOPE_ACTIONS" :key="action.id" class="pb-2 text-center">{{ action.label }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="resource in SCOPE_RESOURCES" :key="resource.id" class="table-row-hover">
                <td class="py-2 font-medium text-heading">{{ resource.label }}</td>
                <td v-for="action in SCOPE_ACTIONS" :key="action.id" class="text-center">
                  <input
                    type="checkbox"
                    :checked="hasScope(scopeID(resource.id, action.id))"
                    @change="toggleResource(resource.id, action.id)"
                  />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" type="submit">{{ editing ? 'Save scopes' : 'Create key' }}</button>
        <button v-if="editing" class="btn-ghost" type="button" @click="reset">Cancel</button>
      </div>
    </form>
    <div class="card">
      <div class="table-scroll">
      <table class="data-table">
        <thead class="text-muted"><tr><th class="pb-2">Name</th><th>Prefix</th><th>Scopes</th><th></th></tr></thead>
        <tbody>
          <tr v-for="k in items" :key="k.id" class="table-row-hover">
            <td class="py-2">{{ k.name }}</td>
            <td>{{ k.prefix }}</td>
            <td class="max-w-md text-muted">{{ (k.scopes || []).join(', ') }}</td>
            <td class="text-right">
              <div class="flex justify-end gap-1.5">
                <button class="btn-row btn-row-edit" type="button" @click="edit(k)">
                  <Pencil class="h-3.5 w-3.5" />
                  Edit
                </button>
                <button class="btn-row btn-row-danger" type="button" @click="askRemove(k)">
                  <Trash2 class="h-3.5 w-3.5" />
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
  </div>
</template>
