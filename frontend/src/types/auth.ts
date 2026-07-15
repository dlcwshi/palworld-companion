export type UserRole = 'admin' | 'player'
export type UserStatus = 'pending' | 'active' | 'disabled' | 'rejected' | 'deleted'
export interface User {
  id: number
  username: string | null
  displayName: string
  steamId: string | null
  palworldUserId: string | null
  palworldPlayerId?: string | null
  characterName: string
  accountName: string
  role: UserRole
  status: UserStatus
  createdAt: string
  updatedAt: string
  lastLoginAt: string | null
  lastSeenAt: string | null
  deletedAt?: string | null
  approvedAt: string | null
  approvedBy: number | null
  rejectedAt: string | null
  rejectedBy: number | null
  rejectionReason?: string
}
export interface AuthResponse { authenticated: boolean; user: User | null }
export interface UsersResponse { users: User[] }
export interface SetupStatus { setupRequired: boolean }
