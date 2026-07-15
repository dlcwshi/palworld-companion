import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { User } from '@/types/auth'

export const useAuthStore = defineStore('auth', {
  state: () => ({ ready: false, setupRequired: null as boolean | null, user: null as User | null }),
  getters: { authenticated: (state) => state.user !== null, isAdmin: (state) => state.user?.role === 'admin' },
  actions: {
    async initialize() {
      const setup = await api.setup.status()
      this.setupRequired = setup.setupRequired
      if (!setup.setupRequired) {
        const result = await api.auth.me()
        this.user = result.authenticated ? result.user : null
      } else this.user = null
      this.ready = true
    },
    async refresh() { await this.initialize() },
    async setupAdmin(input: { username: string; displayName: string; password: string; confirmPassword: string }) {
      const result = await api.setup.admin(input)
      this.user = result.user
      this.setupRequired = false
      this.ready = true
    },
    async login(account: string, password: string) {
      const result = await api.auth.login(account, password)
      this.user = result.user
    },
    async logout() { await api.auth.logout(); this.user = null },
  },
})
