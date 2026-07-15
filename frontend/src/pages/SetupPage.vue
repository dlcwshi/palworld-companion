<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { APIError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore(); const route = useRoute(); const router = useRouter()
const username = ref(''); const displayName = ref(''); const password = ref(''); const confirmPassword = ref(''); const busy = ref(false); const error = ref<string | null>(route.query.unavailable === '1' ? '暂时无法读取初始化状态，请稍后重试。' : null)
async function submit() {
  error.value = null
  if (password.value !== confirmPassword.value) { error.value = '两次输入的密码不一致。'; return }
  busy.value = true
  try { await auth.setupAdmin({ username: username.value, displayName: displayName.value, password: password.value, confirmPassword: confirmPassword.value }); await router.replace('/admin/users') }
  catch (value) { error.value = value instanceof APIError ? value.message : '初始化失败，请重试。' }
  finally { busy.value = false }
}
</script>
<template><div class="simple-page auth-page"><p class="eyebrow">INITIAL SETUP</p><h1>创建首任管理员</h1><section class="section-block prose-card"><p>此初始化只能成功一次。创建后只能通过管理员后台或服务器 CLI 恢复管理权限。</p><form class="auth-form" @submit.prevent="submit"><label>管理员用户名<input v-model.trim="username" autocomplete="username" minlength="3" maxlength="64" required /></label><label>显示名称（可选）<input v-model.trim="displayName" maxlength="80" /></label><label>密码<input v-model="password" type="password" autocomplete="new-password" minlength="8" maxlength="128" required /></label><label>确认密码<input v-model="confirmPassword" type="password" autocomplete="new-password" minlength="8" maxlength="128" required /></label><div v-if="error" class="notice danger">{{ error }}</div><button class="primary-button" :disabled="busy">{{ busy ? '正在初始化…' : '创建管理员并登录' }}</button></form></section></div></template>
