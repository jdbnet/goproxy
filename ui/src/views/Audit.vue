<script setup>
import { onMounted, ref } from 'vue'
import api from '@/api/client'

const items = ref([])

async function load() {
  const { data } = await api.get('/audit?limit=200')
  items.value = data || []
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <h1 class="text-xl font-semibold">Audit</h1>
    <div class="card overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Time</th>
            <th>Actor</th>
            <th>Action</th>
            <th>Resource</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="e in items" :key="e.id" class="table-row-hover">
            <td class="py-2 whitespace-nowrap">{{ e.at }}</td>
            <td>{{ e.actor_type }}:{{ e.actor_id }}</td>
            <td>{{ e.action }}</td>
            <td>{{ e.resource }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
