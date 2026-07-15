import type { PlayersResponse, SummaryResponse } from '@/types/server'

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { headers: { Accept: 'application/json' }, cache: 'no-store' })
  if (!response.ok) throw new Error(`Companion returned HTTP ${response.status}`)
  return response.json() as Promise<T>
}
export const api = {
  summary: () => getJSON<SummaryResponse>('/api/v1/server/summary'),
  players: () => getJSON<PlayersResponse>('/api/v1/server/players'),
}
