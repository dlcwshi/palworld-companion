export type UserRole = 'admin' | 'player'
export type UserStatus = 'active' | 'disabled' | 'deleted'
export interface User { id:number; steamId:string; characterName:string; accountName:string; role:UserRole; status:UserStatus; createdAt:string; lastLoginAt:string; lastSeenAt:string|null; deletedAt?:string|null }
export interface AuthResponse { authenticated:boolean; user:User|null }
export interface UsersResponse { users:User[] }
