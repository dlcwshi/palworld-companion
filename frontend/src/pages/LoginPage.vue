<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
const route=useRoute();const auth=useAuthStore();const errors:Record<string,string>={player_offline:'请先进入本 Palworld 服务器并保持在线，然后重新使用 Steam 登录。',palworld_unavailable:'Palworld API 暂时不可用，首次注册未创建账号。',account_disabled:'账号已被禁用，请联系管理员。',account_deleted:'账号已删除，请联系管理员。',invalid_flow:'登录流程已过期或已使用，请重新开始。',steam_verification_failed:'Steam 登录验证失败，请重试。'};const error=computed(()=>typeof route.query.error==='string'?errors[route.query.error]??'登录失败，请重试。':null)
</script>
<template><div class="simple-page auth-page"><p class="eyebrow">STEAM ACCOUNT</p><h1>登录 Companion</h1><section class="section-block prose-card"><h2>Steam 登录即注册</h2><p>首次登录前，请先进入本 Palworld 服务器并保持角色在线。完成绑定后，后续登录不要求角色在线。</p><div v-if="error" class="notice danger">{{ error }}</div><button class="primary-button" type="button" @click="auth.login('/tasks')">使用 Steam 登录</button><small>不需要 Companion 密码，也不需要 Steam Web API Key。</small></section></div></template>
