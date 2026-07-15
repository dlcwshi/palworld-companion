import type { PlayersResponse, SummaryResponse } from '@/types/server'
import type { Task, TaskCreateInput, TaskListResponse, TaskStatus, TaskUpdateInput } from '@/types/tasks'
import type { AuthResponse, SetupStatus, UsersResponse, UserStatus } from '@/types/auth'

export class APIError extends Error {
  constructor(public readonly code: string, message: string, public readonly status: number) { super(message) }
}
async function parse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { code?: string; message?: string; error?: string } | null
    throw new APIError(payload?.code ?? 'request_failed', payload?.message ?? payload?.error ?? `Companion returned HTTP ${response.status}`, response.status)
  }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>
}
async function getJSON<T>(path: string): Promise<T> {
  return parse<T>(await fetch(path, { headers: { Accept: 'application/json' }, cache: 'no-store' }))
}
async function requestJSON<T>(path: string, method: string, body?: unknown): Promise<T> {
  return parse<T>(await fetch(path, { method, headers: { Accept: 'application/json', ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) }))
}
export const api = {
  summary: () => getJSON<SummaryResponse>('/api/v1/server/summary'),
  players: () => getJSON<PlayersResponse>('/api/v1/server/players'),
  setup: {
    status: () => getJSON<SetupStatus>('/api/v1/setup/status'),
    admin: (input: { username: string; displayName: string; password: string; confirmPassword: string }) => requestJSON<AuthResponse>('/api/v1/setup/admin', 'POST', input),
  },
  auth: {
    me: () => getJSON<AuthResponse>('/api/v1/auth/me'),
    login: (account: string, password: string) => requestJSON<AuthResponse>('/api/v1/auth/login', 'POST', { account, password }),
    register: (steamId: string, password: string, confirmPassword: string) => requestJSON<{ status: UserStatus; message: string }>('/api/v1/auth/register', 'POST', { steamId, password, confirmPassword }),
    changePassword: (currentPassword: string, newPassword: string, confirmPassword: string) => requestJSON<void>('/api/v1/auth/change-password', 'POST', { currentPassword, newPassword, confirmPassword }),
    logout: () => requestJSON<void>('/api/v1/auth/logout', 'POST'),
  },
  admin: {
    users: (status = '') => getJSON<UsersResponse>(`/api/v1/admin/users${status ? `?status=${status}` : ''}`),
    action: (id: number, action: 'approve' | 'disable' | 'enable' | 'restore' | 'revoke-sessions') => requestJSON<void>(`/api/v1/admin/users/${id}/${action}`, 'POST'),
    reject: (id: number, reason: string) => requestJSON<void>(`/api/v1/admin/users/${id}/reject`, 'POST', { reason }),
    resetPassword: (id: number, password: string, confirmPassword: string) => requestJSON<void>(`/api/v1/admin/users/${id}/reset-password`, 'POST', { password, confirmPassword }),
    setRole: (id: number, role: 'admin' | 'player') => requestJSON<void>(`/api/v1/admin/users/${id}/role`, 'POST', { role }),
    remove: (id: number) => requestJSON<void>(`/api/v1/admin/users/${id}`, 'DELETE'),
  },
  tasks: {
    list: (status: TaskStatus | 'all' = 'all', limit = 100, scope = 'visible') => getJSON<TaskListResponse>(`/api/v1/tasks?status=${status}&limit=${limit}&scope=${scope}`),
    create: (input: TaskCreateInput) => requestJSON<Task>('/api/v1/tasks', 'POST', input),
    update: (id: number, input: TaskUpdateInput) => requestJSON<Task>(`/api/v1/tasks/${id}`, 'PATCH', input),
    remove: (id: number) => requestJSON<void>(`/api/v1/tasks/${id}`, 'DELETE'),
  },
}
