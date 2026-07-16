<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Player, PlayersResponse } from '@/types/server'

const props = defineProps<{ response: PlayersResponse | null; loading: boolean }>()
type Filter = 'all' | 'online' | 'offline'
const filter = ref<Filter>('all')
const known = computed(() => props.response?.currentStatusKnown === true)
const players = computed(() => props.response?.players ?? [])
const filtered = computed(() => filter.value === 'all' ? players.value : players.value.filter((player) => player.status === filter.value))
watch(known, (value) => { if (!value) filter.value = 'all' })

const value = (input: number | null | undefined, suffix = '') => input == null ? '—' : `${Math.round(input)}${suffix}`
const coordinates = (player: Player) => {
  if (!player.position) return '—'
  const parts = [player.position.x, player.position.y]
  if (player.position.z != null) parts.push(player.position.z)
  return parts.map((item) => item.toFixed(1)).join(', ')
}
const fullTime = (raw: string) => new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(raw))
const relativeTime = (raw: string) => {
  const date = new Date(raw)
  const minutes = Math.floor(Math.max(0, Date.now() - date.getTime()) / 60_000)
  if (minutes < 1) return '刚刚在线'
  if (minutes < 60) return `${minutes} 分钟前在线`
  const now = new Date()
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
  if (date >= yesterday && date < new Date(now.getFullYear(), now.getMonth(), now.getDate())) {
    return `昨天 ${new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' }).format(date)} 在线`
  }
  return `${new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date)} 在线`
}
const confirmedAt = computed(() => props.response?.updatedAt ? fullTime(props.response.updatedAt) : '尚未完成过完整同步')
const emptyText = computed(() => {
  if (!props.response?.available) return '尚未发现任何玩家'
  if (filter.value === 'online') return '当前没有玩家在线'
  if (filter.value === 'offline') return '当前没有离线玩家'
  return '尚未发现玩家'
})
</script>

<template>
  <section class="section-block player-section">
    <div class="section-heading">
      <div><p class="eyebrow">PLAYERS</p><h2>玩家</h2></div>
      <span class="count-pill">{{ response?.counts.total ?? 0 }}</span>
    </div>
    <div v-if="loading && !response" class="player-state-message">正在读取玩家状态…</div>
    <div v-else-if="response && !response.currentStatusKnown" class="notice warning roster-warning">当前状态暂时无法确认，以下为持久化名册中的历史状态。</div>
    <p class="roster-confirmed">最后一次状态确认：{{ confirmedAt }}</p>
    <div class="player-counts" aria-label="玩家统计">
      <span>全部 {{ response?.counts.total ?? 0 }}</span>
      <span>在线 {{ response?.counts.currentOnline ?? '—' }}</span>
      <span>离线 {{ response?.counts.currentOffline ?? '—' }}</span>
      <span v-if="!known">上次已知在线 {{ response?.counts.lastKnownOnline ?? 0 }}</span>
    </div>
    <div class="player-filter" role="group" aria-label="筛选玩家状态">
      <button type="button" :class="{ active: filter === 'all' }" @click="filter = 'all'">全部</button>
      <button type="button" :class="{ active: filter === 'online' }" :disabled="!known" @click="filter = 'online'">在线</button>
      <button type="button" :class="{ active: filter === 'offline' }" :disabled="!known" @click="filter = 'offline'">离线</button>
    </div>
    <div v-if="filtered.length" class="player-list">
      <article v-for="(player, index) in filtered" :key="`${player.name}-${player.lastOnlineAt}-${index}`" class="player-row roster-player-row">
        <div class="avatar" :class="player.status">{{ player.name.slice(0, 1).toUpperCase() || '?' }}</div>
        <div class="player-main">
          <div class="player-name-line"><strong>{{ player.name || '—' }}</strong><span class="presence-badge" :class="player.status">{{ player.status === 'online' ? '在线' : player.status === 'offline' ? '离线' : '状态未知' }}</span></div>
          <template v-if="player.status === 'online'"><span>坐标 {{ coordinates(player) }}</span></template>
          <template v-else-if="player.status === 'offline'"><time :datetime="player.lastOnlineAt" :title="fullTime(player.lastOnlineAt)">{{ relativeTime(player.lastOnlineAt) }}</time></template>
          <template v-else>
            <span>上次已知{{ player.lastKnownStatus === 'online' ? '在线' : '离线' }}</span>
            <time :datetime="player.lastOnlineAt" :title="fullTime(player.lastOnlineAt)">最后发现在线：{{ relativeTime(player.lastOnlineAt) }}</time>
          </template>
        </div>
        <div class="player-meta"><strong>Lv. {{ value(player.level) }}</strong><span v-if="player.status === 'online'">{{ value(player.ping, ' ms') }}</span></div>
      </article>
    </div>
    <p v-else-if="!loading || response" class="empty-state">{{ emptyText }}</p>
  </section>
</template>
