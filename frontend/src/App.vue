<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { NConfigProvider, NMessageProvider, NDialogProvider, NGlobalStyle, NButton, darkTheme, useOsTheme } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import Sidebar from './components/Sidebar.vue'
import PlayerBar from './components/PlayerBar.vue'
import FullPlayerOverlay from './components/FullPlayerOverlay.vue'
import { useSystemStore } from './stores/system'
import { usePlayerStore } from './stores/player'
import { usePreferencesStore } from './stores/preferences'

const route = useRoute()
const systemStore = useSystemStore()
const playerStore = usePlayerStore()
const preferencesStore = usePreferencesStore()

// 结合系统与用户偏好的暗色模式判定
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

// 苹果设计规范自定义 Naive UI 亮暗双态主题定制（Premium Design）
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
      borderRadius: '11px',
      successColor: '#34c759',
      warningColor: '#ffcc00',
      errorColor: '#ff3b30',
      fontSize: '14px',
      textColor1: isDarkVal ? '#ececf0' : '#1d1d1f',
      textColor2: isDarkVal ? 'rgba(236, 236, 240, 0.82)' : 'rgba(29, 29, 31, 0.82)',
      textColor3: isDarkVal ? 'rgba(236, 236, 240, 0.52)' : 'rgba(29, 29, 31, 0.52)',
    },
    Button: {
      borderRadiusMedium: '9999px',
      borderRadiusSmall: '8px',
      borderRadiusLarge: '9999px',
      fontWeightMedium: '500',
    },
    Card: {
      borderRadius: '18px'
    },
    Dialog: {
      borderRadius: '18px'
    },
    Modal: {
      borderRadius: '18px'
    },
    Input: {
      borderRadius: '11px',
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
      borderRadius: '8px',
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
  // 从本地存储恢复全局缩放设置并应用
  try {
    const savedScale = localStorage.getItem('2fmusic_ui_scale')
    if (savedScale) {
      document.documentElement.style.setProperty('--ui-scale', savedScale)
    }
  } catch (e) {
    console.error(e)
  }

  // 延时加载以防资源未加载完
  systemStore.fetchSystemStatus()
  preferencesStore.fetchPreferences()

  // 绑定全局按键事件
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
    <n-global-style />
    <n-message-provider placement="bottom">
      <n-dialog-provider>
        <div class="app-layout" :class="{ 'has-custom-bg': preferencesStore.customBgEnabled }">
          <!-- 全局高颜值自定义磨砂毛玻璃背景底层 -->
          <div 
            v-if="preferencesStore.customBgEnabled && preferencesStore.bgUrl" 
            class="global-custom-bg" 
            :style="{ backgroundImage: `url(${preferencesStore.bgUrl})` }"
          ></div>
          <!-- 侧边栏 -->
          <aside class="app-sidebar" :class="{ 'sidebar-active': isSidebarActive }">
            <Sidebar @close-sidebar="isSidebarActive = false" />
          </aside>

          <!-- 遮罩层 (移动端侧边栏展开时) -->
          <div 
            v-if="isSidebarActive" 
            class="sidebar-overlay" 
            @click="isSidebarActive = false"
          ></div>

          <!-- 主工作区 -->
          <div class="app-main">
            <!-- 移动端顶部标题栏 -->
            <header class="mobile-top-bar">
              <n-button circle text class="mobile-menu-btn" @click="isSidebarActive = true">
                <template #icon>
                  <SvgIcon name="menu" />
                </template>
              </n-button>
              <div class="mobile-page-title">{{ pageTitle }}</div>
              <div class="mobile-placeholder"></div> <!-- 占位平衡 -->
            </header>

            <!-- 路由视图滚动区 -->
            <div class="main-scroll">
              <router-view />
            </div>

            <!-- 底部播放栏 -->
            <PlayerBar @open-lyrics="showLyricsOverlay = true" />
          </div>

          <!-- 全屏歌词覆层 -->
          <FullPlayerOverlay 
            :show="showLyricsOverlay" 
            @close="showLyricsOverlay = false" 
          />
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
/* 响应式全局缩放支持 */
:root {
  --ui-scale: 1.0;
}

body {
  zoom: var(--ui-scale);
}

#app {
  transform-origin: top left;
  /* 我们可以根据 --ui-scale 动态调节主要字体比例等，配合 naive ui 配置 */
}

/* 侧边遮罩 */
.sidebar-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: calc(100vw / var(--ui-scale, 1.0));
  height: calc(100vh / var(--ui-scale, 1.0));
  background-color: rgba(0, 0, 0, 0.3);
  z-index: 9;
}

/* 移动端顶栏 */
.mobile-top-bar {
  display: none;
  height: 52px;
  background: var(--bg-sidebar);
  backdrop-filter: var(--blur-sidebar);
  -webkit-backdrop-filter: var(--blur-sidebar);
  border-bottom: 1px solid var(--border-main);
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  box-sizing: border-box;
  z-index: 5;
  flex-shrink: 0;
}

.mobile-menu-btn {
  background: transparent;
  border: none;
  color: var(--ink);
  font-size: var(--font-size-xl);
  cursor: pointer;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 移动端占位平衡元素 */
.mobile-placeholder {
  width: 32px;
}

.mobile-page-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--ink);
}

@media (max-width: 768px) {
  .mobile-top-bar {
    display: flex;
  }
}
</style>
