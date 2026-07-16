<script setup lang="ts">
defineProps<{ name: 'home' | 'tasks' | 'breeding' | 'map' | 'account' }>()
</script>

<template>
  <svg class="app-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
    <template v-if="name === 'home'">
      <path d="M3 10.5 12 3l9 7.5" />
      <path d="M5 9.5V21h14V9.5M9 21v-7h6v7" />
    </template>
    <template v-else-if="name === 'tasks'">
      <path d="M9 6h11M9 12h11M9 18h11" />
      <path d="m3.5 6 1.3 1.3L7 4.8M3.5 12l1.3 1.3L7 10.8M3.5 18l1.3 1.3L7 16.8" />
    </template>
    <template v-else-if="name === 'breeding'">
      <path d="M8 3c4 3 4 6 0 9s-4 6 0 9M16 3c-4 3-4 6 0 9s4 6 0 9" />
      <path d="M8 6h8M7 12h10M8 18h8" />
    </template>
    <template v-else-if="name === 'map'">
      <path d="m3 6 6-3 6 3 6-3v15l-6 3-6-3-6 3Z" />
      <path d="M9 3v15M15 6v15" />
    </template>
    <template v-else>
      <circle cx="12" cy="8" r="4" />
      <path d="M4.5 21a7.5 7.5 0 0 1 15 0" />
    </template>
  </svg>
</template>
