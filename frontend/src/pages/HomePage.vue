<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import MetricCard from '@/components/MetricCard.vue'
import PlayerList from '@/components/PlayerList.vue'
import { useServerStore } from '@/stores/server'
import { useTaskStore } from '@/stores/tasks'
import { useAuthStore } from '@/stores/auth'
import { groupHomeTasks } from '@/utils/homeTasks'

const store = useServerStore()
const taskStore = useTaskStore()
const auth = useAuthStore()
const browserOnline = ref(navigator.onLine)
let timer: number | undefined
const server = computed(() => store.summary?.server)
const isOffline = computed(() => !browserOnline.value)
const taskGroups = computed(() => groupHomeTasks(taskStore.tasks, auth.user?.id, 5))
const status = computed(() => {
  if (store.backendError) return { label: 'Companion 后端不可用', tone: 'danger' }
  if (!store.summary) return { label: '正在读取服务器状态', tone: 'warning' }
  if (!store.summary.available) return { label: 'Palworld API 不可用', tone: 'danger' }
  if (store.summary.stale) return { label: '服务器状态暂时无法确认', tone: 'warning' }
  return { label: '服务器在线', tone: 'online' }
})
const pageNotice = computed(() => {
  const playerStateUnknown = store.players?.currentStatusKnown === false && store.players.stale
  if (isOffline.value && playerStateUnknown) return { tone: 'warning', text: '设备离线，因此无法刷新玩家状态，正在显示上次结果。' }
  if (isOffline.value) return { tone: 'warning', text: '当前设备离线。' }
  if (store.backendError) return { tone: 'danger', text: 'Companion 后端暂时无法连接，正在显示上次结果。' }
  if (playerStateUnknown) return { tone: 'warning', text: '当前玩家状态暂时无法确认，正在显示上次结果。' }
  if (store.summary?.error) return { tone: 'danger', text: '服务器状态暂时无法确认，正在显示上次结果。' }
  return null
})
const number = (input: number | null | undefined, digits = 0) => input == null ? '—' : input.toFixed(digits)
const uptime = computed(() => {
  const seconds = server.value?.uptimeSeconds
  if (seconds == null) return '—'
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
})
const updatedAt = computed(() => {
  const raw = store.summary?.updatedAt
  return raw ? new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(raw)) : '—'
})
const loadHomeTasks = async () => {
  if (auth.authenticated) await taskStore.load('pending', 100, 'visible')
}
const refreshHome = async () => {
  await Promise.all([store.refresh(), loadHomeTasks()])
}
const onVisibility = () => { if (document.visibilityState === 'visible') void store.refresh() }
const onOnline = () => {
  browserOnline.value = true
  void refreshHome()
}
const onOffline = () => { browserOnline.value = false }

onMounted(() => {
  void auth.refresh().then(loadHomeTasks)
  void store.refresh()
  timer = window.setInterval(() => { if (document.visibilityState === 'visible') void store.refresh() }, 5000)
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('online', onOnline)
  window.addEventListener('offline', onOffline)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisibility)
  window.removeEventListener('online', onOnline)
  window.removeEventListener('offline', onOffline)
})
</script>

<template>
  <div class="home-page">
    <header class="hero-header">
      <div class="hero-brand"><p class="eyebrow">SELF-HOSTED · V0.4.2 DEV</p><h1><span>Palworld</span><br class="title-break" /><em>Companion</em></h1></div>
      <button class="refresh-button" type="button" :disabled="store.refreshing" aria-label="刷新服务器状态" @click="refreshHome"><span :class="{ spinning: store.refreshing }">↻</span></button>
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
      <div class="online-total">
        <strong>{{ number(server?.onlinePlayers) }}</strong>
        <span>/ {{ number(server?.maxPlayers) }} {{ server?.onlinePlayersKnown ? '在线' : '当前在线状态未知' }}</span>
      </div>
    </section>

    <div v-if="pageNotice" class="notice" :class="pageNotice.tone">{{ pageNotice.text }}</div>

    <section class="metrics-grid">
      <MetricCard label="服务器 FPS" :value="number(server?.fps, 1)" accent />
      <MetricCard label="运行时间" :value="uptime" />
      <MetricCard label="世界天数" :value="number(server?.worldDays)" />
      <MetricCard label="基地数量" :value="number(server?.baseCount)" />
    </section>

    <section class="task-summary section-block">
      <div class="section-heading">
        <div><p class="eyebrow">TASKS</p><h2>任务</h2></div>
        <span class="count-pill">{{ taskGroups.totalIncompleteTasks }}</span>
      </div>
      <div v-if="!auth.authenticated" class="empty-state">登录后查看个人与共享任务。<RouterLink class="secondary-button" to="/login">登录</RouterLink></div>
      <template v-else>
        <div v-if="taskStore.error" class="notice danger">{{ taskStore.error }}</div>
        <div v-if="taskStore.loading && taskGroups.totalIncompleteTasks === 0" class="empty-state">正在加载任务…</div>
        <div v-else-if="taskGroups.totalIncompleteTasks === 0" class="empty-state">还没有未完成任务</div>
        <template v-else>
          <div v-if="taskGroups.personalTotal" class="home-task-group">
            <h3>个人任务 · {{ taskGroups.personalTotal }}</h3>
            <ul class="task-summary-list">
              <li v-for="task in taskGroups.personalTasks" :key="task.id"><RouterLink class="home-task-link" to="/tasks"><span>○</span><strong>{{ task.title }}</strong></RouterLink></li>
            </ul>
          </div>
          <div v-if="taskGroups.sharedTotal" class="home-task-group">
            <h3>共享任务 · {{ taskGroups.sharedTotal }}</h3>
            <ul class="task-summary-list">
              <li v-for="task in taskGroups.sharedTasks" :key="task.id"><RouterLink class="home-task-link" to="/tasks"><span>○</span><strong>{{ task.title }}</strong></RouterLink></li>
            </ul>
          </div>
        </template>
        <div class="task-summary-actions">
          <RouterLink to="/tasks?new=1" class="secondary-button">＋ 新建任务</RouterLink>
          <RouterLink to="/tasks" class="text-link">查看全部 →</RouterLink>
        </div>
      </template>
    </section>

    <PlayerList :response="store.players" :loading="store.refreshing" />
    <footer class="update-footer">
      <span>服务器摘要更新 {{ updatedAt }}</span>
    </footer>
  </div>
</template>
