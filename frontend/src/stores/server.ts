import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { PlayersResponse, SummaryResponse } from '@/types/server'

export const useServerStore = defineStore('server', {
  state: () => ({
    summary: null as SummaryResponse | null,
    players: null as PlayersResponse | null,
    refreshing: false,
    backendError: null as string | null,
    lastAttemptAt: null as Date | null,
  }),
  getters: {
    isStale: (state) => Boolean(state.summary?.stale || state.players?.stale),
    upstreamError: (state) => state.summary?.error ?? state.players?.error ?? null,
  },
  actions: {
    async refresh() {
      if (this.refreshing) return
      this.refreshing = true
      const [summary, players] = await Promise.allSettled([api.summary(), api.players()])
      this.backendError = null
      if (summary.status === 'fulfilled') this.summary = summary.value
      else this.backendError = 'Companion 后端不可用'
      if (players.status === 'fulfilled') this.players = players.value
      else this.backendError = 'Companion 后端不可用'
      this.lastAttemptAt = new Date()
      this.refreshing = false
    },
  },
})