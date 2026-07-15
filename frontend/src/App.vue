<script setup lang="ts">
const comingSoon = () => window.alert('此功能将在后续版本推出')
</script>

<template>
  <div class="app-shell">
    <main class="page-shell"><RouterView /></main>
    <nav class="bottom-nav" aria-label="主要导航">
      <RouterLink to="/" class="nav-item" aria-label="首页"><span class="nav-icon">⌂</span><span>首页</span></RouterLink>
      <button class="nav-item muted" type="button" @click="comingSoon"><span class="nav-icon">◇</span><span>制作</span></button>
      <button class="nav-item muted" type="button" @click="comingSoon"><span class="nav-icon">∞</span><span>配种</span></button>
      <button class="nav-item muted" type="button" @click="comingSoon"><span class="nav-icon">⌖</span><span>地图</span></button>
      <RouterLink to="/settings" class="nav-item" aria-label="我的"><span class="nav-icon">○</span><span>我的</span></RouterLink>
    </nav>
  </div>
</template>