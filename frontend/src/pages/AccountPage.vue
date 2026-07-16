<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, APIError } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { APP_VERSION } from '@/version'

const auth = useAuthStore()
const router = useRouter()
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const busy = ref(false)
const error = ref<string | null>(null)
const success = ref<string | null>(null)
const passwordOpen = ref(false)
const currentPasswordInput = ref<HTMLInputElement | null>(null)

const displayName = computed(() => auth.user?.displayName || auth.user?.characterName || auth.user?.username || auth.user?.steamId || '当前用户')
const loginAccount = computed(() => auth.user?.username || auth.user?.characterName || auth.user?.steamId || '—')

const clearPasswordForm = () => {
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  error.value = null
}
const openPasswordForm = async () => {
  success.value = null
  error.value = null
  passwordOpen.value = true
  await nextTick()
  currentPasswordInput.value?.focus()
}
const closePasswordForm = () => {
  passwordOpen.value = false
  clearPasswordForm()
}
const togglePasswordForm = () => passwordOpen.value ? closePasswordForm() : void openPasswordForm()

async function changePassword() {
  error.value = null
  success.value = null
  if (newPassword.value !== confirmPassword.value) {
    error.value = '两次输入的新密码不一致。'
    return
  }
  busy.value = true
  try {
    await api.auth.changePassword(currentPassword.value, newPassword.value, confirmPassword.value)
    clearPasswordForm()
    passwordOpen.value = false
    success.value = '密码已修改，其他登录会话已撤销。'
  } catch (value) {
    error.value = value instanceof APIError ? value.message : '修改密码失败。'
  } finally {
    busy.value = false
  }
}
async function logout() {
  await auth.logout()
  await router.replace('/login')
}

onBeforeUnmount(clearPasswordForm)
</script>

<template>
  <div class="simple-page account-page">
    <header class="account-header">
      <p class="eyebrow">ACCOUNT</p>
      <h1>我的账号</h1>
      <p>管理你的本地登录信息和账号设置。</p>
    </header>

    <div v-if="success" class="notice success account-notice" role="status">{{ success }}</div>

    <section class="account-card" aria-labelledby="account-overview-title">
      <div class="account-identity">
        <div class="account-avatar" aria-hidden="true">{{ displayName.slice(0, 1).toUpperCase() }}</div>
        <div class="account-name-block">
          <p id="account-overview-title" class="account-name">{{ displayName }}</p>
          <div class="account-badges">
            <span class="account-badge role">{{ auth.user?.role === 'admin' ? '管理员' : '玩家' }}</span>
            <span class="account-badge status">{{ auth.user?.status }}</span>
          </div>
        </div>
      </div>
      <dl class="account-details">
        <div><dt>登录账号</dt><dd>{{ loginAccount }}</dd></div>
        <div v-if="auth.user?.steamId"><dt>SteamID64</dt><dd>{{ auth.user.steamId }}</dd></div>
        <div v-if="auth.user?.characterName"><dt>绑定角色</dt><dd>{{ auth.user.characterName }}</dd></div>
      </dl>
    </section>

    <section class="settings-card" aria-label="账号操作">
      <RouterLink v-if="auth.isAdmin" class="settings-action" to="/admin/users">
        <span><strong>管理用户</strong><small>审批玩家和管理本地账号</small></span><span class="settings-chevron" aria-hidden="true">›</span>
      </RouterLink>
      <button class="settings-action" type="button" :aria-expanded="passwordOpen" aria-controls="password-settings" @click="togglePasswordForm">
        <span><strong>修改密码</strong><small>更新本地登录密码</small></span><span class="settings-chevron" aria-hidden="true">{{ passwordOpen ? '⌃' : '›' }}</span>
      </button>
      <button class="settings-action danger-action" type="button" @click="logout">
        <span><strong>退出登录</strong><small>结束当前设备上的登录会话</small></span><span class="settings-chevron" aria-hidden="true">›</span>
      </button>
    </section>

    <section v-if="passwordOpen" id="password-settings" class="password-panel" aria-labelledby="password-settings-title">
      <div class="password-panel-heading">
        <div><p class="eyebrow">SECURITY</p><h2 id="password-settings-title">修改密码</h2></div>
        <button class="text-button" type="button" :disabled="busy" @click="closePasswordForm">收起</button>
      </div>
      <form class="auth-form" @submit.prevent="changePassword">
        <label for="current-password">当前密码</label>
        <input id="current-password" ref="currentPasswordInput" v-model="currentPassword" type="password" autocomplete="current-password" required />
        <label for="new-password">新密码</label>
        <input id="new-password" v-model="newPassword" type="password" autocomplete="new-password" minlength="8" maxlength="128" required />
        <label for="confirm-password">确认新密码</label>
        <input id="confirm-password" v-model="confirmPassword" type="password" autocomplete="new-password" minlength="8" maxlength="128" required />
        <div v-if="error" class="notice danger password-error" role="alert">{{ error }}</div>
        <div class="password-actions">
          <button class="secondary-button" type="button" :disabled="busy" @click="closePasswordForm">取消</button>
          <button class="primary-button" type="submit" :disabled="busy">{{ busy ? '正在保存…' : '保存新密码' }}</button>
        </div>
      </form>
    </section>

    <p class="account-version">Palworld Companion {{ APP_VERSION }}</p>
  </div>
</template>
