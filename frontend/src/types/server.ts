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
  maxPlayers: number | null
  uptimeSeconds: number | null
  worldDays: number | null
  baseCount: number | null
}
export interface PlayersResponse {
  available: boolean
  cached: boolean
  stale: boolean
  updatedAt: string | null
  players: Player[]
  error: string | null
}
export interface Player {
  name: string
  level: number | null
  ping: number | null
  position: { x: number; y: number } | null
}
