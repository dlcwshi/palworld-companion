<script setup lang="ts">
defineProps<{ label: string; value: string; accent?: boolean }>()
</script>

<template>
  <article class="metric-card" :class="{ accent }">
    <span>{{ label }}</span>
    <strong>{{ value }}</strong>
  </article>
</template>
