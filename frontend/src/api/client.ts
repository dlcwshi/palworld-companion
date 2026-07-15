import type { PlayersResponse, SummaryResponse } from '@/types/server'
import type { Task, TaskCreateInput, TaskListResponse, TaskStatus, TaskUpdateInput } from '@/types/tasks'
import type { AuthResponse, UsersResponse } from '@/types/auth'

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: 'application/json' }, cache: 'no-store' })
  if (!response.ok) throw new Error(`Companion returned HTTP ${response.status}`)
  return response.json() as Promise<T>
}

async function requestJSON<T>(path: string, method: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    headers: { Accept: 'application/json', ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null
    throw new Error(payload?.error ?? `Companion returned HTTP ${response.status}`)
  }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>
}
export const api = {
  summary: () => getJSON<SummaryResponse>('/api/v1/server/summary'),
  players: () => getJSON<PlayersResponse>('/api/v1/server/players'),
  auth: { me: () => getJSON<AuthResponse>('/api/v1/auth/me'), logout: () => requestJSON<void>('/api/v1/auth/logout', 'POST') },
  admin: { users: () => getJSON<UsersResponse>('/api/v1/admin/users'), action: (id:number, action:'disable'|'enable'|'restore'|'revoke-sessions') => requestJSON<void>(`/api/v1/admin/users/${id}/${action}`, 'POST'), remove: (id:number) => requestJSON<void>(`/api/v1/admin/users/${id}`, 'DELETE') },
  tasks: {
    list: (status: TaskStatus | 'all' = 'all', limit = 100, scope = 'visible') => getJSON<TaskListResponse>(`/api/v1/tasks?status=${status}&limit=${limit}&scope=${scope}`),
    create: (input: TaskCreateInput) => requestJSON<Task>('/api/v1/tasks', 'POST', input),
    update: (id: number, input: TaskUpdateInput) => requestJSON<Task>(`/api/v1/tasks/${id}`, 'PATCH', input),
    remove: (id: number) => requestJSON<void>(`/api/v1/tasks/${id}`, 'DELETE'),
  },
}
