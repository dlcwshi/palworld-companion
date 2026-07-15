<script setup lang="ts">
import type { Player } from '@/types/server'

defineProps<{ players: Player[] }>()
const value = (input: number | null | undefined, suffix = '') => input == null ? '—' : `${Math.round(input)}${suffix}`
const coordinates = (player: Player) => player.position ? `${player.position.x.toFixed(1)}, ${player.position.y.toFixed(1)}` : '—'
</script>

<template>
  <section class="section-block">
    <div class="section-heading">
      <div><p class="eyebrow">PLAYERS</p><h2>在线玩家</h2></div>
      <span class="count-pill">{{ players.length }}</span>
    </div>
    <div v-if="players.length" class="player-list">
      <article v-for="(player, index) in players" :key="`${player.name}-${index}`" class="player-row">
        <div class="avatar">{{ player.name.slice(0, 1).toUpperCase() || '?' }}</div>
        <div class="player-main"><strong>{{ player.name || '—' }}</strong><span>坐标 {{ coordinates(player) }}</span></div>
        <div class="player-meta"><strong>Lv. {{ value(player.level) }}</strong><span>{{ value(player.ping, ' ms') }}</span></div>
      </article>
    </div>
    <p v-else class="empty-state">当前没有可显示的在线玩家</p>
  </section>
</template>