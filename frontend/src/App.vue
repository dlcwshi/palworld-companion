<script setup lang="ts">
import AppIcon from '@/components/AppIcon.vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const comingSoon = () => window.alert('此功能将在后续版本推出')
</script>

<template>
  <div class="app-shell">
    <main class="page-shell"><RouterView /></main>
    <nav v-if="auth.setupRequired !== true" class="bottom-nav" aria-label="主要导航">
      <RouterLink to="/" class="nav-item" aria-label="首页"><AppIcon name="home" /><span>首页</span></RouterLink>
      <RouterLink to="/tasks" class="nav-item" aria-label="任务"><AppIcon name="tasks" /><span>任务</span></RouterLink>
      <button class="nav-item muted" type="button" aria-label="配种，即将推出" @click="comingSoon"><AppIcon name="breeding" /><span>配种</span></button>
      <button class="nav-item muted" type="button" aria-label="地图，即将推出" @click="comingSoon"><AppIcon name="map" /><span>地图</span></button>
      <RouterLink to="/account" class="nav-item" aria-label="我的"><AppIcon name="account" /><span>我的</span></RouterLink>
    </nav>
  </div>
</template>
