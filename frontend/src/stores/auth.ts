import { defineStore } from 'pinia'
import { api } from '@/api/client'
import type { User } from '@/types/auth'

export const useAuthStore=defineStore('auth',{state:()=>({ready:false,user:null as User|null}),getters:{authenticated:(s)=>s.user!==null,isAdmin:(s)=>s.user?.role==='admin'},actions:{async refresh(){try{const result=await api.auth.me();this.user=result.authenticated?result.user:null}finally{this.ready=true}},login(returnTo='/tasks'){window.location.assign(`/api/v1/auth/steam?returnTo=${encodeURIComponent(returnTo)}`)},async logout(){await api.auth.logout();this.user=null}}})
