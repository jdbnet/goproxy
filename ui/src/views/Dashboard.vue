<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import api from '@/api/client'
import Sparkline from '@/components/Sparkline.vue'
import { formatBytes, formatLatency, formatUptime, trafficLabel, trafficTitle } from '@/lib/bytes'

const live = ref({})
const history = ref([])
const limitation = ref('')
const status = ref([])
const rows = computed(() => [...status.value].sort((a, b) => {
  if (!!a.healthy !== !!b.healthy) return a.healthy ? 1 : -1
  const an = (a.name || a.backend || '').toLowerCase()
  const bn = (b.name || b.backend || '').toLowerCase()
  if (an !== bn) return an.localeCompare(bn)
  return (a.target || '').localeCompare(b.target || '')
}))
let timer

async function refresh() {
  const [s, h, b] = await Promise.all([
    api.get('/stats'),
    api.get('/stats/history'),
    api.get('/backends/status').catch(() => ({ data: [] })),
  ])
  live.value = s.data
  history.value = h.data.history || []
  limitation.value = h.data.limitation || ''
  status.value = b.data || []
}

onMounted(async () => {
  await refresh()
  timer = setInterval(refresh, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-xl font-semibold">Overview</h1>
      <p class="mt-1 text-sm text-muted">
        Up {{ formatUptime(live.uptime_seconds) }}
        <span v-if="limitation"> · {{ limitation }}</span>
      </p>
    </div>
    <div class="grid gap-4 md:grid-cols-4">
      <div class="card">
        <div class="text-xs text-muted">Requests (window)</div>
        <div class="mt-1 text-2xl font-semibold">{{ live.requests ?? 0 }}</div>
        <p class="mt-1 text-xs text-muted">{{ live.requests_total ?? 0 }} since start</p>
        <Sparkline class="mt-2" :points="history" field="requests" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Errors</div>
        <div class="mt-1 text-2xl font-semibold">{{ live.errors ?? 0 }}</div>
        <Sparkline class="mt-2" :points="history" field="errors" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Connections</div>
        <div class="mt-1 text-2xl font-semibold">{{ live.connections ?? 0 }}</div>
        <Sparkline class="mt-2" :points="history" field="connections" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Latency</div>
        <div class="mt-1 text-2xl font-semibold">{{ formatLatency(live.latency_ms) }}</div>
        <p class="mt-1 text-xs text-muted">Time to first byte</p>
        <Sparkline class="mt-2" :points="history" field="latency_ms" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Traffic in</div>
        <div class="mt-1 text-2xl font-semibold">{{ formatBytes(live.bytes_in_total) }}</div>
        <p class="mt-1 text-xs text-muted">{{ formatBytes(live.bytes_in) }} this window</p>
        <Sparkline class="mt-2" :points="history" field="bytes_in" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Traffic out</div>
        <div class="mt-1 text-2xl font-semibold">{{ formatBytes(live.bytes_out_total) }}</div>
        <p class="mt-1 text-xs text-muted">{{ formatBytes(live.bytes_out) }} this window</p>
        <Sparkline class="mt-2" :points="history" field="bytes_out" />
      </div>
      <div class="card">
        <div class="text-xs text-muted">Backends up</div>
        <div class="mt-1 text-2xl font-semibold">{{ live.backends_healthy ?? 0 }}/{{ live.backends_total ?? 0 }}</div>
      </div>
    </div>
    <div class="card">
      <h2 class="mb-3 text-sm font-medium text-muted">Backend health</h2>
      <div class="table-scroll">
      <table class="data-table">
        <thead class="text-muted">
          <tr>
            <th class="pb-2">Backend</th>
            <th class="pb-2">Target</th>
            <th class="pb-2">Role</th>
            <th class="pb-2">Health</th>
            <th class="pb-2">Latency</th>
            <th class="pb-2">Traffic</th>
            <th class="pb-2">Conns</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in rows" :key="s.backend + '\n' + s.target" class="table-row-hover">
            <td class="py-2">{{ s.name || s.backend }}</td>
            <td>{{ s.target }}</td>
            <td>{{ s.role }}</td>
            <td>
              <span :class="s.healthy ? 'badge badge-running' : 'badge badge-error'">
                {{ s.healthy ? 'up' : 'down' }}
              </span>
            </td>
            <td
              class="whitespace-nowrap"
              :title="s.probe_ms ? 'Health probe' : 'No health check'"
            >{{ formatLatency(s.latency_ms) }}</td>
            <td :title="trafficTitle(s)">{{ trafficLabel(s) }}</td>
            <td>{{ s.conns }}</td>
          </tr>
          <tr v-if="!rows.length">
            <td colspan="7" class="py-4 text-muted">No backends configured.</td>
          </tr>
        </tbody>
      </table>
      </div>
    </div>
  </div>
</template>
