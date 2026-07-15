<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import MetricCard from '@/components/MetricCard.vue'
import PlayerList from '@/components/PlayerList.vue'
import { useServerStore } from '@/stores/server'

const store = useServerStore()
let timer: number | undefined
const server = computed(() => store.summary?.server)
const players = computed(() => store.players?.players ?? [])
const isOffline = computed(() => !navigator.onLine)
const status = computed(() => {
  if (store.backendError) return { label: 'Companion 后端不可用', tone: 'danger' }
  if (!store.summary?.available) return { label: 'Palworld API 不可用', tone: 'danger' }
  if (store.isStale) return { label: '显示缓存数据', tone: 'warning' }
  return { label: '服务器在线', tone: 'online' }
})
const number = (input: number | null | undefined, digits = 0) => input == null ? '—' : input.toFixed(digits)
const uptime = computed(() => {
  const seconds = server.value?.uptimeSeconds
  if (seconds == null) return '—'
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
})
const updatedAt = computed(() => {
  const raw = store.summary?.updatedAt ?? store.players?.updatedAt
  return raw ? new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(raw)) : '—'
})
const onVisibility = () => { if (document.visibilityState === 'visible') void store.refresh() }
const onOnline = () => void store.refresh()

onMounted(() => {
  void store.refresh()
  timer = window.setInterval(() => { if (document.visibilityState === 'visible') void store.refresh() }, 5000)
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('online', onOnline)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
  window.removeEventListener('online', onOnline)
})
</script>

<template>
  <div class="home-page">
    <header class="hero-header">
      <div><p class="eyebrow">SELF-HOSTED · V0.1</p><h1>Palworld<br /><em>Companion</em></h1></div>
      <button class="refresh-button" type="button" :disabled="store.refreshing" aria-label="刷新" @click="store.refresh"><span :class="{ spinning: store.refreshing }">↻</span></button>
    </header>

    <section class="status-card">
      <div class="status-top">
        <span class="status-badge" :class="status.tone"><i />{{ status.label }}</span>
        <span class="server-version">{{ server?.version ?? '版本未知' }}</span>
      </div>
      <div class="server-title">
        <div class="signal-orbit"><span>●</span></div>
        <div><p>当前服务器</p><h2>{{ server?.name ?? '等待服务器响应' }}</h2></div>
      </div>
      <p v-if="server?.description" class="description">{{ server.description }}</p>
      <div class="online-total"><strong>{{ number(server?.onlinePlayers) }}</strong><span>/ {{ number(server?.maxPlayers) }} 在线</span></div>
    </section>

    <div v-if="isOffline" class="notice warning">当前离线，正在显示上一次加载的页面。</div>
    <div v-if="store.backendError" class="notice danger">{{ store.backendError }}。已保留上次数据显示。</div>
    <div v-else-if="store.upstreamError" class="notice danger">{{ store.upstreamError }}<span v-if="store.isStale">，以下为上次成功数据。</span></div>

    <section class="metrics-grid">
      <MetricCard label="服务器 FPS" :value="number(server?.fps, 1)" accent />
      <MetricCard label="运行时间" :value="uptime" />
      <MetricCard label="世界天数" :value="number(server?.worldDays)" />
      <MetricCard label="基地数量" :value="number(server?.baseCount)" />
    </section>

    <PlayerList :players="players" />
    <footer class="update-footer">
      <span>最后更新 {{ updatedAt }}</span>
      <span v-if="store.summary?.cached" class="cache-label">{{ store.summary.stale ? '缓存 · 已过期' : '缓存命中' }}</span>
    </footer>
  </div>
</template>