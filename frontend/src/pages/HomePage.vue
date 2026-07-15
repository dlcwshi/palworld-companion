<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import MetricCard from '@/components/MetricCard.vue'
import PlayerList from '@/components/PlayerList.vue'
import { useServerStore } from '@/stores/server'
import { useTaskStore } from '@/stores/tasks'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api/client'
import type { Task } from '@/types/tasks'

const store = useServerStore()
const taskStore = useTaskStore()
const auth = useAuthStore()
const mineTasks=ref<Task[]>([])
const sharedTasks=ref<Task[]>([])
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
  void auth.refresh().then(async()=>{if(auth.authenticated){const [mine,shared]=await Promise.all([api.tasks.list('pending',5,'mine'),api.tasks.list('pending',5,'shared')]);mineTasks.value=mine.tasks;sharedTasks.value=shared.tasks;void taskStore.load('pending',5,'visible')}})
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
      <div><p class="eyebrow">SELF-HOSTED · V0.2 DEV</p><h1>Palworld<br /><em>Companion</em></h1></div>
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

    <section class="task-summary section-block">
      <div class="section-heading">
        <div><p class="eyebrow">TONIGHT</p><h2>今晚任务</h2></div>
        <span class="count-pill">{{ taskStore.total }}</span>
      </div>
      <div v-if="!auth.authenticated" class="empty-state">登录后查看个人与共享任务。<button class="secondary-button" @click="auth.login('/')">使用 Steam 登录</button></div>
      <template v-else>
        <div v-if="taskStore.error" class="notice danger">{{ taskStore.error }}</div>
        <div class="task-scope-summary"><strong>我的未完成 {{ mineTasks.length }}</strong><ul><li v-for="task in mineTasks" :key="task.id">{{ task.title }}</li></ul></div>
        <div class="task-scope-summary"><strong>共享未完成 {{ sharedTasks.length }}</strong><ul><li v-for="task in sharedTasks" :key="task.id">{{ task.title }}</li></ul></div>
        <div v-if="taskStore.loading" class="empty-state">正在加载今晚任务…</div>
        <div v-else-if="taskStore.tasks.length === 0" class="empty-state">今晚还没有任务，先添加一件想完成的事。</div>
        <ul v-else class="task-summary-list">
          <li v-for="task in taskStore.tasks" :key="task.id"><span>○</span><strong>{{ task.title }}</strong></li>
        </ul>
        <div class="task-summary-actions">
          <RouterLink to="/tasks?new=1" class="secondary-button">＋ 新建任务</RouterLink>
          <RouterLink to="/tasks" class="text-link">查看全部 →</RouterLink>
        </div>
      </template>
    </section>

    <PlayerList :players="players" />
    <footer class="update-footer">
      <span>最后更新 {{ updatedAt }}</span>
      <span v-if="store.summary?.cached" class="cache-label">{{ store.summary.stale ? '缓存 · 已过期' : '缓存命中' }}</span>
    </footer>
  </div>
</template>
