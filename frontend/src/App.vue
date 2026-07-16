<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore()
const comingSoon = () => window.alert('此功能将在后续版本推出')
</script>

<template>
  <div class="app-shell">
    <main class="page-shell"><RouterView /></main>
    <nav v-if="auth.setupRequired !== true" class="bottom-nav" aria-label="主要导航">
      <RouterLink to="/" class="nav-item" aria-label="首页"><span class="nav-icon">⌂</span><span>首页</span></RouterLink>
      <RouterLink to="/tasks" class="nav-item" aria-label="任务"><span class="nav-icon">✓</span><span>任务</span></RouterLink>
      <button class="nav-item muted" type="button" @click="comingSoon"><span class="nav-icon">∞</span><span>配种</span></button>
      <button class="nav-item muted" type="button" @click="comingSoon"><span class="nav-icon">⌖</span><span>地图</span></button>
      <RouterLink to="/account" class="nav-item" aria-label="我的"><span class="nav-icon">○</span><span>我的</span></RouterLink>
    </nav>
  </div>
</template>
