<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { NConfigProvider, NMessageProvider, NDialogProvider, NGlobalStyle, NButton, NProgress, darkTheme, useOsTheme } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import PlayerBar from './components/PlayerBar.vue'
import FullPlayerOverlay from './components/FullPlayerOverlay.vue'
import { useSystemStore } from './stores/system'
import { usePlayerStore } from './stores/player'
import { usePreferencesStore } from './stores/preferences'
import { useFavoritesStore } from './stores/favorites'

const route = useRoute()
const systemStore = useSystemStore()
const playerStore = usePlayerStore()
const preferencesStore = usePreferencesStore()
const favoritesStore = useFavoritesStore()

const isReady = ref(false)

// 首屏数据就绪真实进度与文本计算
const loadedCount = ref(0)
const totalCount = 6
const progress = computed(() => {
  return Math.min(100, Math.round((loadedCount.value / totalCount) * 100))
})

const loadingText = computed(() => {
  const p = progress.value
  if (p < 25) return '正在建立安全连接...'
  if (p < 50) return '正在同步系统配置...'
  if (p < 75) return '正在唤醒本地歌曲...'
  if (p < 100) return '正在构建收藏索引...'
  return '音乐就绪，开启旋律！'
})

// 1. 同步首屏极速无闪烁设置初始化，杜绝尺寸和主题模式的瞬间闪烁
try {
  const savedScale = localStorage.getItem('2fmusic_ui_scale')
  if (savedScale) {
    document.documentElement.style.setProperty('--ui-scale', savedScale)
  }

  const localTheme = localStorage.getItem('2fmusic_theme_mode') || 'system'
  preferencesStore.themeMode = localTheme as any

  const root = document.documentElement
  root.classList.remove('theme-light', 'theme-dark')
  if (localTheme !== 'system') {
    root.classList.add(`theme-${localTheme}`)
  }
} catch (e) {
  console.error('初始化无闪烁设置发生异常:', e)
}

// 2. 异步并行加载系统状态与全部偏好，逐个监听核心任务resolve情况，同步推进真实百分比
const initApp = async () => {
  loadedCount.value = 0

  const tasks = [
    systemStore.fetchSystemStatus(),
    preferencesStore.fetchPreferences(),
    systemStore.fetchSongs(),
    favoritesStore.fetchPlaylists(),
    systemStore.fetchNeteaseConfig(),
    systemStore.fetchNeteaseUserStatus()
  ]

  try {
    await Promise.all(
      tasks.map(async (task) => {
        try {
          await task
        } catch (e) {
          console.error('核心预加载任务失败:', e)
        } finally {
          loadedCount.value++
        }
      })
    )
  } catch (e) {
    console.error('预加载任务流遇到未知错误:', e)
  } finally {
    // 进度达到 100% 后，给用户保留 250ms 的短暂视觉反馈时间，看清进度和欢迎语后优雅切入主页
    await new Promise(resolve => setTimeout(resolve, 250))
    isReady.value = true
  }
}

initApp()

// 结合系统偏好与用户设置判定暗色模式
const osThemeRef = useOsTheme()
const isDark = computed(() => {
  if (preferencesStore.themeMode === 'system') {
    return osThemeRef.value === 'dark'
  }
  return preferencesStore.themeMode === 'dark'
})
const theme = computed(() => isDark.value ? darkTheme : null)

// 全局键盘快捷键控制逻辑
const handleKeyDown = (e: KeyboardEvent) => {
  // 若用户处于输入框中，忽略快捷键
  const activeEl = document.activeElement
  if (
    activeEl &&
    (activeEl.tagName === 'INPUT' ||
      activeEl.tagName === 'TEXTAREA' ||
      activeEl.getAttribute('contenteditable') !== null)
  ) {
    return
  }

  switch (e.code) {
    case 'Space':
      e.preventDefault()
      playerStore.togglePlay()
      break
    case 'ArrowLeft':
      e.preventDefault()
      playerStore.seek(playerStore.currentTime - 5)
      break
    case 'ArrowRight':
      e.preventDefault()
      playerStore.seek(playerStore.currentTime + 5)
      break
    case 'ArrowUp':
      e.preventDefault()
      playerStore.setVolume(playerStore.volume + 0.05)
      break
    case 'ArrowDown':
      e.preventDefault()
      playerStore.setVolume(playerStore.volume - 0.05)
      break
  }
}

// 鼠标点击后自动释放焦点，消除残留高亮并防止空格键误触二次点击
const handleMouseUp = () => {
  const activeEl = document.activeElement as HTMLElement
  if (activeEl && (activeEl.tagName === 'BUTTON' || activeEl.closest('button'))) {
    activeEl.blur()
  }
}

// 移动端侧边栏展开折叠
const isSidebarActive = ref(false)

// 歌词全屏显示控制
const showLyricsOverlay = ref(false)

const pageTitle = computed(() => {
  if (route.path.startsWith('/favorites/')) return '收藏夹详情'
  const map: Record<string, string> = {
    '/': '本地音乐',
    '/favorites': '我的收藏',
    '/history': '播放记录',
    '/mounts': '目录管理',
    '/netease': '网易下载',
    '/upload': '上传音乐',
    '/settings': '系统设置'
  }
  return map[route.path] || '2FMusic'
})

// Naive UI 亮暗双态主题定制（Apple 设计规范）
const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const isDarkVal = isDark.value
  const primary = isDarkVal ? '#2997ff' : '#007aff'
  const primaryHover = isDarkVal ? '#47a7ff' : '#3395ff'
  const primaryPressed = isDarkVal ? '#1485ff' : '#0062cc'

  return {
    common: {
      primaryColor: primary,
      primaryColorHover: primaryHover,
      primaryColorPressed: primaryPressed,
      primaryColorSuppl: primary,
      borderRadius: 'var(--radius-md)',
      successColor: '#34c759',
      warningColor: '#ffcc00',
      errorColor: '#ff3b30',
      fontSize: 'var(--font-size-md)',
      textColor1: isDarkVal ? '#ececf0' : '#1d1d1f',
      textColor2: isDarkVal ? 'rgba(236, 236, 240, 0.82)' : 'rgba(29, 29, 31, 0.82)',
      textColor3: isDarkVal ? 'rgba(236, 236, 240, 0.52)' : 'rgba(29, 29, 31, 0.52)',
    },
    Button: {
      borderRadiusMedium: '9999px',
      borderRadiusSmall: 'var(--radius-sm)',
      borderRadiusLarge: '9999px',
      fontWeightMedium: '500',
    },
    Card: {
      borderRadius: 'var(--radius-lg)'
    },
    Dialog: {
      borderRadius: 'var(--radius-lg)'
    },
    Modal: {
      borderRadius: 'var(--radius-lg)'
    },
    Input: {
      borderRadius: 'var(--radius-md)',
      borderHover: `1px solid ${primary}`,
      borderFocus: `1px solid ${primary}`,
    },
    Checkbox: {
      colorChecked: primary,
      borderChecked: `1px solid ${primary}`,
      checkMarkColor: '#ffffff',
    },
    Radio: {},
    Tag: {
      borderRadius: 'var(--radius-sm)',
    },
    Slider: {
      handleColor: '#ffffff',
      fillColor: primary,
      fillColorHover: primaryHover,
      railColor: isDarkVal ? 'rgba(255, 255, 255, 0.12)' : 'rgba(0, 0, 0, 0.08)',
      railColorHover: isDarkVal ? 'rgba(255, 255, 255, 0.18)' : 'rgba(0, 0, 0, 0.12)'
    },
    Switch: {
      railColorActive: primary
    }
  }
})

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
  window.addEventListener('mouseup', handleMouseUp)

  // 全局拉起并初始化 WebSocket 实时通信信道
  systemStore.initWebSocket()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
  window.removeEventListener('mouseup', handleMouseUp)
})
</script>

<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-global-style />
    <n-message-provider placement="bottom">
      <n-dialog-provider>
        <!-- 首屏数据就绪过渡遮罩 -->
        <div v-if="!isReady"
          class="fixed inset-0 flex flex-col items-center justify-center bg-canvas-parchment text-ink z-99999 gap-5 select-none">
          <!-- 进度环 -->
          <div class="relative w-20 h-20 flex items-center justify-center">
            <n-progress type="circle" :percentage="progress" :stroke-width="7" :show-indicator="false"
              class="w-full! h-full! transition-all duration-300" />
            <!-- 环中心等宽高对比度等分比数字 -->
            <div class="absolute inset-0 flex items-center justify-center">
              <span class="text-sm font-semibold tracking-tight text-ink font-mono">{{ progress }}%</span>
            </div>
          </div>
          <span
            class="text-[13px] font-medium tracking-wider text-body-muted animate-pulse mt-1 h-[20px] transition-all duration-300">
            {{ loadingText }}
          </span>
        </div>

        <div v-else class="app-layout" :class="{ 'has-custom-bg': preferencesStore.customBgEnabled }">
          <!-- 自定义磨砂毛玻璃背景 -->
          <div v-if="preferencesStore.customBgEnabled && preferencesStore.bgUrl" class="global-custom-bg"
            :style="{ backgroundImage: `url(${preferencesStore.bgUrl})` }"></div>
          <!-- 侧边栏 -->
          <aside
            class="w-(--sidebar-width) h-full flex flex-col shrink-0 z-10 border-r border-border-main box-border bg-sidebar backdrop-saturate-180 backdrop-blur-sidebar max-md:fixed max-md:-left-(--sidebar-width) max-md:transition-transform max-md:duration-300 max-md:bg-canvas max-md:bg-none"
            :class="[
              isSidebarActive ? 'max-md:translate-x-(--sidebar-width)' : '',
              preferencesStore.customBgEnabled ? 'max-md:bg-white/92 max-md:backdrop-blur-[35px] max-md:backdrop-saturate-180 dark:max-md:bg-[#16171d]/92' : ''
            ]">
            <Sidebar @close-sidebar="isSidebarActive = false" />
          </aside>

          <div v-if="isSidebarActive"
            class="fixed top-0 left-0 w-[calc(100vw/var(--ui-scale,1))] h-[calc(100vh/var(--ui-scale,1))] bg-black/30 z-9"
            @click="isSidebarActive = false"></div>

          <!-- 主工作区 -->
          <div
            class="relative z-1 flex-1 flex flex-col min-h-0 overflow-hidden bg-app-main backdrop-blur-main transition-colors duration-300">
            <!-- 移动端顶部标题栏 -->
            <header
              class="hidden max-md:flex h-[52px] bg-sidebar backdrop-blur-sidebar border-b border-border-main items-center justify-between px-4 box-border z-5 shrink-0">
              <n-button circle text
                class="bg-transparent border-none text-ink text-xl cursor-pointer w-8 h-8 flex items-center justify-center rounded-full hover:bg-black/5"
                @click="isSidebarActive = true">
                <template #icon>
                  <SvgIcon name="menu" />
                </template>
              </n-button>
              <div class="text-[15px] font-semibold text-ink">{{ pageTitle }}</div>
              <div class="w-8"></div>
            </header>

            <div class="main-scroll flex-1 min-h-0 overflow-y-auto p-[24px_28px] max-md:p-[16px_12px] box-border">
              <router-view />
            </div>

            <PlayerBar @open-lyrics="showLyricsOverlay = true" />
          </div>

          <!-- 全屏歌词覆层 -->
          <FullPlayerOverlay :show="showLyricsOverlay" @close="showLyricsOverlay = false" />
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>


<style>
/* 响应式全局缩放支持 */
:root {
  --ui-scale: 1.0;
  --sidebar-width: clamp(180px, 18vw, 240px);
}

body {
  zoom: var(--ui-scale);
}

#app {
  transform-origin: top left;
}
</style>
