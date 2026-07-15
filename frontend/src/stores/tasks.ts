import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { Task, TaskCreateInput, TaskStatus, TaskUpdateInput } from '@/types/tasks'

function message(error: unknown) {
  return error instanceof Error ? error.message : '任务操作失败'
}

export const useTaskStore = defineStore('tasks', {
  state: () => ({
    tasks: [] as Task[],
    total: 0,
    status: 'all' as TaskStatus | 'all',
    scope: 'visible',
    loading: false,
    saving: false,
    error: null as string | null,
  }),
  getters: {
    pendingTasks: (state) => state.tasks.filter((task) => task.status === 'pending'),
  },
  actions: {
    async load(status: TaskStatus | 'all' = this.status, limit = 100, scope = this.scope) {
      this.loading = true
      this.error = null
      try {
        const result = await api.tasks.list(status, limit, scope)
        this.tasks = result.tasks
        this.total = result.total
        this.status = status
        this.scope = scope
      } catch (error) {
        this.error = message(error)
      } finally {
        this.loading = false
      }
    },
    async create(input: TaskCreateInput) {
      if (this.saving) return false
      this.saving = true
      this.error = null
      try {
        await api.tasks.create(input)
        await this.load(this.status)
        return true
      } catch (error) {
        this.error = message(error)
        return false
      } finally {
        this.saving = false
      }
    },
    async update(id: number, input: TaskUpdateInput) {
      if (this.saving) return false
      this.saving = true
      this.error = null
      try {
        await api.tasks.update(id, input)
        await this.load(this.status)
        return true
      } catch (error) {
        this.error = message(error)
        return false
      } finally {
        this.saving = false
      }
    },
    async remove(id: number) {
      if (this.saving) return false
      this.saving = true
      this.error = null
      try {
        await api.tasks.remove(id)
        await this.load(this.status)
        return true
      } catch (error) {
        this.error = message(error)
        return false
      } finally {
        this.saving = false
      }
    },
  },
})
