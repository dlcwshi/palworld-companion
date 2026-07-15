<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
const auth=useAuthStore()
const safety = [
  'Palworld REST API 凭据仅保存在后端配置中',
  '前端只访问 Companion 的 /api/v1 接口',
  'Palworld 身份绑定仅使用实时 players 结果，公开玩家字段仍经过过滤',
]
</script>

<template>
  <div class="simple-page">
    <p class="eyebrow">ABOUT & SETUP</p>
    <h1>我的 Companion</h1>
    <section class="section-block prose-card"><h2>当前账号</h2><template v-if="auth.user"><p>{{ auth.user.characterName }} · {{ auth.user.role==='admin'?'管理员':'玩家' }}</p><RouterLink v-if="auth.isAdmin" class="text-link" to="/admin/users">管理用户 →</RouterLink><button class="secondary-button" type="button" @click="auth.logout()">退出登录</button></template><template v-else><p>使用 Steam 登录，不需要 Companion 密码。</p><button class="primary-button" type="button" @click="auth.login('/settings')">使用 Steam 登录</button></template></section>
    <section class="section-block prose-card"><h2>连接说明</h2><p>当前实例由服务器管理员自托管。若状态不可用，请检查 Companion 后端与 Palworld REST API 的局域网连接。</p></section>
    <section class="section-block prose-card"><h2>安全边界</h2><ul><li v-for="item in safety" :key="item">{{ item }}</li></ul></section>
    <section class="section-block prose-card"><h2>版本路线</h2><p>当前版本 0.2.0-dev · 已支持 Steam 账号和个人/共享任务。制作、配种与地图仍在后续规划中。</p></section>
  </div>
</template>