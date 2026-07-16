export interface SummaryResponse {
  available: boolean
  cached: boolean
  stale: boolean
  updatedAt: string | null
  server: ServerSummary | null
  error: string | null
}
export interface ServerSummary {
  name: string
  version: string | null
  description: string | null
  fps: number | null
  onlinePlayers: number | null
  onlinePlayersKnown: boolean
  maxPlayers: number | null
  uptimeSeconds: number | null
  worldDays: number | null
  baseCount: number | null
}
export interface PlayerCounts {
  total: number
  currentOnline: number | null
  currentOffline: number | null
  lastKnownOnline: number
  lastKnownOffline: number
}
export interface PlayersResponse {
  available: boolean
  cached: boolean
  stale: boolean
  currentStatusKnown: boolean
  updatedAt: string | null
  counts: PlayerCounts
  players: Player[]
  error: string | null
}
export type PlayerStatus = 'online' | 'offline' | 'unknown'
export interface Player {
  name: string
  level: number
  status: PlayerStatus
  lastKnownStatus: 'online' | 'offline'
  lastOnlineAt: string
  ping: number | null
  position: { x: number; y: number; z?: number } | null
}
