<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { Check, ChevronsUpDown, Search } from '@lucide/vue'

const props = defineProps({
  modelValue: { type: String, default: '' },
  options: { type: Array, default: () => [] },
  placeholder: { type: String, default: 'Select' },
  allowEmpty: { type: Boolean, default: false },
  emptyLabel: { type: String, default: 'None' },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue'])

const open = ref(false)
const query = ref('')
const root = ref(null)
const searchEl = ref(null)

const selected = computed(() => props.options.find((o) => o.value === props.modelValue))

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return props.options.filter((o) => {
    if (!q) return true
    const hay = [o.value, o.label, o.hint, o.search].filter(Boolean).join(' ').toLowerCase()
    return hay.includes(q)
  })
})

function choose(value) {
  emit('update:modelValue', value)
  open.value = false
  query.value = ''
}

async function toggle() {
  if (props.disabled) return
  open.value = !open.value
  if (open.value) {
    query.value = ''
    await nextTick()
    searchEl.value?.focus()
  }
}

function onDoc(e) {
  if (root.value && !root.value.contains(e.target)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', onDoc))
onUnmounted(() => document.removeEventListener('mousedown', onDoc))
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      class="input-field flex items-center justify-between gap-2 text-left"
      :disabled="disabled"
      @click="toggle"
    >
      <span v-if="selected" class="min-w-0 truncate">
        <span class="text-heading">{{ selected.label }}</span>
        <span v-if="selected.hint" class="ml-2 text-muted">{{ selected.hint }}</span>
      </span>
      <span v-else-if="allowEmpty && !modelValue" class="text-muted">{{ emptyLabel }}</span>
      <span v-else class="text-muted">{{ placeholder }}</span>
      <ChevronsUpDown class="h-4 w-4 shrink-0 text-muted" />
    </button>
    <div
      v-if="open"
      class="absolute z-30 mt-1 w-full overflow-hidden rounded-lg border border-default bg-surface-raised shadow-lg"
    >
      <div class="flex items-center gap-2 border-b border-default px-3 py-2">
        <Search class="h-4 w-4 text-muted" />
        <input
          ref="searchEl"
          v-model="query"
          class="w-full bg-transparent text-sm outline-none"
          placeholder="Search by name, address, or domain"
          @keydown.esc.prevent="open = false"
        />
      </div>
      <ul class="max-h-56 overflow-auto py-1">
        <li v-if="allowEmpty">
          <button
            type="button"
            class="flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-accent/10"
            @click="choose('')"
          >
            <span class="text-muted">{{ emptyLabel }}</span>
            <Check v-if="!modelValue" class="h-4 w-4 text-accent" />
          </button>
        </li>
        <li v-for="o in filtered" :key="o.value">
          <button
            type="button"
            class="flex w-full items-start justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-accent/10"
            @click="choose(o.value)"
          >
            <span class="min-w-0">
              <span class="block truncate text-heading">{{ o.label }}</span>
              <span v-if="o.hint" class="block truncate text-xs text-muted">{{ o.hint }}</span>
            </span>
            <Check v-if="o.value === modelValue" class="mt-0.5 h-4 w-4 shrink-0 text-accent" />
          </button>
        </li>
        <li v-if="!filtered.length" class="px-3 py-2 text-sm text-muted">No matches</li>
      </ul>
    </div>
  </div>
</template>
