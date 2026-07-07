<template>
  <div class="flex flex-col flex-1 min-h-0 w-full h-full relative overflow-hidden">
    <iframe 
      :src="foliaUrl" 
      class="w-full h-full border-none absolute inset-0 bg-transparent"
      allow="autoplay; clipboard-write"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useSystemStore } from '../stores/system'

const systemStore = useSystemStore()

const foliaUrl = computed(() => {
  const base = './folia/index.html'
  const api = systemStore.neteaseConfig.api_base
  if (api) {
    return `${base}?netease_api=${encodeURIComponent(api)}`
  }
  return base
})
</script>

<style scoped>
iframe {
  width: 100%;
  height: 100%;
  border: none;
}
</style>
