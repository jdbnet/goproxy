<script setup>
import { onMounted, ref } from 'vue'
import { Trash2 } from '@lucide/vue'
import api from '@/api/client'

const items = ref([])
const form = ref({ username: '', password: '', role: 'viewer' })
const error = ref('')

async function load() {
  const { data } = await api.get('/users')
  items.value = data || []
}

async function save() {
  error.value = ''
  try {
    await api.post('/users', form.value)
    form.value = { username: '', password: '', role: 'viewer' }
    await load()
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  }
}

async function remove(id) {
  await api.delete(`/users/${id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Users</h1>
      <p class="mt-1 text-sm text-muted">Dashboard logins. Roles limit what someone can change.</p>
    </div>
    <p v-if="error" class="rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-2 text-sm text-red-600 dark:text-red-300">{{ error }}</p>
    <form class="card grid gap-3 md:grid-cols-4" @submit.prevent="save">
      <input v-model="form.username" class="input-field" placeholder="Username" required />
      <input v-model="form.password" class="input-field" type="password" placeholder="Password" required />
      <select v-model="form.role" class="input-field">
        <option value="admin">Admin</option>
        <option value="operator">Operator</option>
        <option value="viewer">Viewer</option>
      </select>
      <button class="btn-primary" type="submit">Add user</button>
    </form>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted"><tr><th class="pb-2">User</th><th>Role</th><th>Created</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in items" :key="u.id" class="table-row-hover">
            <td class="py-2">{{ u.username }}</td>
            <td>{{ u.role }}</td>
            <td>{{ u.created_at }}</td>
            <td class="text-right">
              <button class="btn-row btn-row-danger" type="button" @click="remove(u.id)">
                <Trash2 class="h-3.5 w-3.5" />
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
