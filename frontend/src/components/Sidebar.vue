<template>
  <div class="flex flex-col h-full p-6 pb-[calc(24px+env(safe-area-inset-bottom))] box-border">
    <!-- 品牌区 -->
    <div class="flex items-center gap-3 mb-8 pl-2">
      <a href="https://github.com/yuexps/2FMusic" target="_blank" class="text-ink no-underline flex items-center transition-opacity duration-150 hover:opacity-80">
        <SvgIcon name="github" size="24" class="w-6 h-6" />
      </a>
      <span class="font-display text-xl font-semibold tracking-[-0.5px] text-ink">2FMusic</span>
    </div>

    <!-- 侧边导航 -->
    <nav class="flex-1">
      <ul class="list-none p-0 m-0 flex flex-col gap-1">
        <li 
          v-for="item in menuItems" 
          :key="item.path"
          :class="[
            'flex items-center gap-3 px-3.5 py-2.5 rounded-lg cursor-pointer text-sm font-normal transition-all duration-150',
            currentPath === item.path 
              ? 'bg-menu-active text-primary dark:text-primary-on-dark font-semibold' 
              : 'text-ink hover:bg-menu-hover hover:text-primary dark:hover:text-primary-on-dark'
          ]"
          @click="navigate(item.path)"
        >
          <SvgIcon :name="item.icon" class="text-base w-5 text-center" />
          <span>{{ item.label }}</span>
        </li>
      </ul>
    </nav>

    <!-- 扫描进度与状态底栏 -->
    <div v-if="systemStore.status.scanning || systemStore.status.is_scraping" class="glass-card p-3 rounded-xl box-border mt-auto">
      <div class="text-xs font-semibold text-primary mb-2">
        <span>{{ systemStore.status.scanning ? '正在扫描音乐库...' : '正在自动刮削元数据...' }}</span>
      </div>
      <div class="flex justify-between text-[11px] text-body-muted mb-1.5">
        <span>进度: {{ systemStore.status.processed }} / {{ systemStore.status.total }}</span>
        <span class="font-semibold text-primary">{{ progressPercent }}%</span>
      </div>
      <n-progress
        type="line"
        :percentage="progressPercent"
        :show-indicator="false"
        processing
        size="small"
        class="mb-1.5"
      />
      <div class="text-[10px] text-body-muted whitespace-nowrap overflow-hidden text-ellipsis text-left" :title="systemStore.status.current_file" style="direction: rtl;">
        {{ systemStore.status.current_file || '准备中...' }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useSystemStore } from '../stores/system'
import { NProgress } from 'naive-ui'

const router = useRouter()
const route = useRoute()
const systemStore = useSystemStore()

const menuItems = [
  { path: '/', label: '本地音乐', icon: 'music' },
  { path: '/favorites', label: '我的收藏', icon: 'heart-o' },
  { path: '/history', label: '播放记录', icon: 'history' },
  { path: '/mounts', label: '目录管理', icon: 'network' },
  { path: '/netease', label: '网易下载', icon: 'cloud-download' },
  { path: '/upload', label: '上传音乐', icon: 'cloud-upload' },
  { path: '/settings', label: '设置', icon: 'cog' }
]

const currentPath = computed(() => {
  // 收藏夹详情页时高亮"我的收藏"
  if (route.path.startsWith('/favorites')) {
    return '/favorites'
  }
  return route.path
})

const navigate = (path: string) => {
  router.push(path)
  // 移动端点击菜单后自动收起侧边栏
  emit('close-sidebar')
}

const emit = defineEmits(['close-sidebar'])

const progressPercent = computed(() => {
  const total = systemStore.status.total
  const processed = systemStore.status.processed
  if (!total) return 0
  return Math.min(100, Math.round((processed / total) * 100))
})
</script>
