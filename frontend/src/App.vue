<script setup>
import { computed, onMounted, ref } from 'vue'

const games = ref([])
const selectedGame = ref('')
const nodeAddr = ref('127.0.0.1:51820')
const status = ref({ running: false, gameID: '', nodeAddr: '', uptimeSecs: 0, packetsSent: 0 })
const latency = ref({ currentMS: 0, minMS: 0, maxMS: 0, avgMS: 0 })
const error = ref('')

const api = computed(() => window.go?.main?.App)

async function refresh() {
  if (!api.value) return
  games.value = await api.value.GetGames()
  status.value = await api.value.GetStatus()
  latency.value = await api.value.GetLatency()
  if (!selectedGame.value && games.value.length > 0) {
    selectedGame.value = games.value[0].id
  }
}

async function start() {
  error.value = ''
  try {
    await api.value.StartAcceleration(selectedGame.value, nodeAddr.value)
    await refresh()
  } catch (err) {
    error.value = String(err)
  }
}

async function stop() {
  error.value = ''
  try {
    await api.value.StopAcceleration()
    await refresh()
  } catch (err) {
    error.value = String(err)
  }
}

onMounted(() => {
  refresh()
  setInterval(refresh, 1000)
})
</script>

<template>
  <main class="shell">
    <section class="toolbar">
      <label>
        Game
        <select v-model="selectedGame">
          <option v-for="game in games" :key="game.id" :value="game.id">
            {{ game.name }}
          </option>
        </select>
      </label>
      <label>
        Node
        <input v-model="nodeAddr" placeholder="host:port" />
      </label>
      <button :disabled="status.running || !selectedGame || !nodeAddr" @click="start">Start</button>
      <button :disabled="!status.running" @click="stop">Stop</button>
    </section>

    <section class="status">
      <div>
        <span>State</span>
        <strong>{{ status.running ? 'Running' : 'Stopped' }}</strong>
      </div>
      <div>
        <span>Game</span>
        <strong>{{ status.gameID || '-' }}</strong>
      </div>
      <div>
        <span>Uptime</span>
        <strong>{{ status.uptimeSecs }}s</strong>
      </div>
      <div>
        <span>Latency</span>
        <strong>{{ latency.currentMS }} ms</strong>
      </div>
    </section>

    <p v-if="error" class="error">{{ error }}</p>
  </main>
</template>
