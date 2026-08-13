<script setup>
import { ref } from 'vue'
import api from '@/api/client'

defineProps({
  compact: { type: Boolean, default: false },
})

const current = ref('')
const next = ref('')
const confirm = ref('')
const error = ref('')
const msg = ref('')
const saving = ref(false)

async function save() {
  error.value = ''
  msg.value = ''
  if (!current.value || !next.value) {
    error.value = 'Enter the current and new password'
    return
  }
  if (next.value !== confirm.value) {
    error.value = 'New passwords do not match'
    return
  }
  saving.value = true
  try {
    await api.post('/auth/password', {
      current_password: current.value,
      new_password: next.value,
    })
    current.value = ''
    next.value = ''
    confirm.value = ''
    msg.value = 'Password updated'
  } catch (e) {
    error.value = e.response?.data?.error || e.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <form :class="compact ? 'space-y-2' : 'card space-y-3'" @submit.prevent="save">
    <div v-if="!compact">
      <h2 class="text-sm font-medium text-heading">Change password</h2>
      <p class="mt-1 text-sm text-muted">Updates the password for the account you are signed in as.</p>
    </div>
    <p v-if="error" class="text-xs text-red-600 dark:text-red-300">{{ error }}</p>
    <p v-if="msg" class="text-xs text-accent">{{ msg }}</p>
    <input v-model="current" class="input-field" type="password" autocomplete="current-password" placeholder="Current password" required />
    <input v-model="next" class="input-field" type="password" autocomplete="new-password" placeholder="New password" required />
    <input v-model="confirm" class="input-field" type="password" autocomplete="new-password" placeholder="Confirm new password" required />
    <button class="btn-primary w-full" type="submit" :disabled="saving">{{ saving ? 'Saving…' : 'Update password' }}</button>
  </form>
</template>
