<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore(); const route = useRoute(); const router = useRouter()
const account = ref(''); const password = ref(''); const busy = ref(false); const error = ref<string | null>(null)
const messages: Record<string,string> = { approval_pending: '申请正在等待管理员审批。', account_disabled: '账号已被禁用，请联系管理员。', application_rejected: '注册申请已被拒绝，请联系管理员。', account_deleted: '账号已删除，请联系管理员。', invalid_credentials: '账号或密码不正确。' }
async function submit() { busy.value = true; error.value = null; try { await auth.login(account.value, password.value); const raw = typeof route.query.returnTo === 'string' ? route.query.returnTo : '/tasks'; await router.replace(raw.startsWith('/') && !raw.startsWith('//') ? raw : '/tasks') } catch(value) { error.value = value instanceof APIError ? messages[value.code] ?? value.message : '登录失败，请重试。' } finally { busy.value = false } }
</script>
<template><div class="simple-page auth-page"><p class="eyebrow">LOCAL ACCOUNT</p><h1>登录 Companion</h1><section class="section-block prose-card"><form class="auth-form" @submit.prevent="submit"><label>账号<input v-model.trim="account" autocomplete="username" placeholder="管理员用户名、角色名或 SteamID64" required /></label><label>密码<input v-model="password" type="password" autocomplete="current-password" maxlength="128" required /></label><div v-if="error" class="notice danger">{{ error }}</div><button class="primary-button" :disabled="busy">{{ busy ? '正在登录…' : '登录' }}</button></form><div class="register-cta"><span>还没有玩家账号？</span><RouterLink class="register-cta-button" to="/register">提交注册申请</RouterLink></div><small>登录完全由本服务器验证，不访问 Steam 社区或 Steam Web API。</small></section></div></template>
