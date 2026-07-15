<script setup lang="ts">
const safety = [
  'Palworld REST API 凭据仅保存在后端配置中',
  '前端只访问 Companion 的 /api/v1 接口',
  'v0.1.0 仅使用 info、metrics、players 三个只读接口',
]
</script>

<template>
  <div class="simple-page">
    <p class="eyebrow">ABOUT & SETUP</p>
    <h1>我的 Companion</h1>
    <section class="section-block prose-card"><h2>连接说明</h2><p>当前实例由服务器管理员自托管。若状态不可用，请检查 Companion 后端与 Palworld REST API 的局域网连接。</p></section>
    <section class="section-block prose-card"><h2>安全边界</h2><ul><li v-for="item in safety" :key="item">{{ item }}</li></ul></section>
    <section class="section-block prose-card"><h2>版本路线</h2><p>当前版本 0.1.0 · 状态首页。任务、制作、配种与地图将在后续版本逐步开放。</p></section>
  </div>
</template>