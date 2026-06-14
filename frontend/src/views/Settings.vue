<template>
  <div class="flex flex-col settings-view">
    <div class="view-header">
      <h1 class="view-title">系统设置</h1>
    </div>

    <div class="settings-list flex flex-col">
      <div v-for="item in settingSections" :key="item.id">
        <!-- 登录与维护 -->
        <div v-if="item.id === 'login'" class="glass-panel p-6 rounded-2xl mb-6 box-border">
          <h3 class="section-title">
            <SvgIcon name="user-shield" /> 登录与维护
          </h3>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">清理应用缓存</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">如果遇到封面显示异常或页面数据加载不出来，请清除应用浏览器缓存。</p>
            </div>
            <n-button round @click="handleClearCache">清除应用缓存</n-button>
          </div>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">退出播放器登录</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">退出当前鉴权账号，清空 Session 会话，重新跳转至登录界面。</p>
            </div>
            <n-button round type="error" @click="handleLogout">退出登录</n-button>
          </div>
        </div>

        <!-- 外观设置 -->
        <div v-else-if="item.id === 'appearance'" class="glass-panel p-6 rounded-2xl mb-6 box-border">
          <h3 class="section-title">
            <SvgIcon name="palette" /> 外观设置
          </h3>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">深浅主题模式</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">切换系统界面的主题色彩风格。</p>
            </div>
            <n-radio-group :value="preferencesStore.themeMode" @update:value="preferencesStore.setThemeMode"
              size="medium" name="themeMode">
              <n-radio-button value="system" label="跟随系统" />
              <n-radio-button value="light" label="浅色" />
              <n-radio-button value="dark" label="深色" />
            </n-radio-group>
          </div>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">全局界面缩放</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">调整页面大小以适配不同分辨率的屏幕。</p>
            </div>
            <n-radio-group :value="uiScalePercent" @update:value="setScale" size="medium" name="uiScale">
              <n-radio-button :value="90" label="缩小 (90%)" />
              <n-radio-button :value="100" label="默认 (100%)" />
              <n-radio-button :value="110" label="放大 (110%)" />
            </n-radio-group>
          </div>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">自定义全屏背景</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">上传你喜爱的图片作为专属背景。</p>
            </div>
            <div class="flex gap-3 items-center">
              <n-button v-if="preferencesStore.bgUrl" round type="error" text @click="handleClearBg">
                清除背景
              </n-button>
              <n-button round type="primary" @click="triggerBgUpload">
                选择图片
              </n-button>
              <input ref="fileInputRef" type="file" accept="image/*" class="hidden" @change="handleBgUpload" />
            </div>
          </div>

        </div>

        <!-- 缓存设置 -->
        <div v-else-if="item.id === 'cache'" class="glass-panel p-6 rounded-2xl mb-6 box-border">
          <h3 class="section-title">
            <SvgIcon name="wifi" /> 缓存设置
          </h3>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">本地缓存封面</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">允许将播放音乐时远程获取的封面缓存在浏览器本地 IndexedDB 中，以提升二次加载效率。</p>
            </div>
            <n-switch v-model:value="cacheCovers" @update:value="saveCacheSettings" />
          </div>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">本地缓存歌词</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">从云端搜刮的歌词直接持久化存入浏览器本地 IndexedDB 中，以提升二次加载效率。</p>
            </div>
            <n-switch v-model:value="cacheLyrics" @update:value="saveCacheSettings" />
          </div>

          <div
            class="flex justify-between items-center py-4 border-b border-border-main flex-wrap gap-4 last:border-none last:pb-0 first:pt-0">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">歌词刮削来源</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">优先从音频文件内嵌元数据提取，或是优先从网络聚合搜索获取。</p>
            </div>
            <n-radio-group :value="preferencesStore.lyricsSourcePref" @update:value="handleLyricsPrefChange"
              size="medium" name="lyricsSource">
              <n-radio-button value="embedded" label="优先内嵌" />
              <n-radio-button value="network" label="优先网络" />
            </n-radio-group>
          </div>
        </div>

        <!-- 网易云下载全局设置 -->
        <div v-else-if="item.id === 'netease'" class="glass-panel p-6 rounded-2xl mb-6 box-border">
          <h3 class="section-title">
            <SvgIcon name="cloud-download" /> 网易云下载全局设置
          </h3>

          <!-- Docker 自动部署 - 已连接 API 时隐藏 -->
          <div v-if="!systemStore.neteaseConfig.api_base"
            class="flex justify-between items-center py-4 border-b border-border-main gap-4 max-sm:flex-col max-sm:items-start">
            <div class="flex-1 min-w-0">
              <h4 class="m-0 mb-1 text-sm font-semibold text-ink">自动部署本地 API (推荐)</h4>
              <p class="m-0 text-xs text-body-muted leading-relaxed">服务器需已安装并启用 Docker，自动拉取并启动 API 容器（端口 23236）。</p>
            </div>
            <div class="shrink-0 min-w-50 flex justify-end max-sm:w-full">
              <div v-if="systemStore.dockerInstallStatus.status === 'running'"
                class="bg-canvas rounded-lg p-[10px_12px] box-border min-w-50">
                <div class="text-xs mb-1.5 text-body-muted">{{ systemStore.dockerInstallStatus.step }}</div>
                <n-progress type="line" :percentage="systemStore.dockerInstallStatus.progress" processing />
              </div>
              <n-button v-else-if="!systemStore.dockerContainerStatus.docker_installed" round disabled>
                需要安装 Docker
              </n-button>
              <n-button v-else round type="primary" @click="handleDockerInstall">
                {{ systemStore.dockerContainerStatus.container_exists ? '启动容器 & 连接' : '一键安装 & 连接' }}
              </n-button>
            </div>
          </div>

          <!-- 手动配置 -->
          <div class="flex flex-col items-start gap-4 border-none py-4">
            <div class="flex flex-col gap-2 w-full">
              <label for="netease-api-input" class="text-xs font-semibold text-body-muted">网易云 API 地址</label>
              <n-input id="netease-api-input" v-model:value="neteaseApi" placeholder="http://localhost:23236" />
            </div>

            <div class="flex flex-col gap-2 w-full">
              <label for="netease-dir-input" class="text-xs font-semibold text-body-muted">歌曲下载保存绝对目录</label>
              <n-input id="netease-dir-input" v-model:value="neteaseDownloadDir"
                placeholder="例如: /vol1/music/NetEase" />
            </div>

            <div class="flex justify-end gap-3 w-full mt-2">
              <n-button v-if="systemStore.neteaseConfig.api_base" round type="error" text @click="disconnectNetease">
                断开 API 连接
              </n-button>
              <n-button round type="primary" @click="saveNeteaseSettings" class="ml-3">
                保存网易云设置
              </n-button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useSystemStore } from '../stores/system'
import { usePreferencesStore } from '../stores/preferences'
import { NSwitch, NButton, NInput, NRadioGroup, NRadioButton, NProgress, useMessage } from 'naive-ui'
import { musicDB } from '../utils/indexedDB'
import { getBaseUrl } from '../utils/path'

const systemStore = useSystemStore()
const preferencesStore = usePreferencesStore()
const message = useMessage()

const settingSections = [
  { id: 'login', label: '登录与维护' },
  { id: 'appearance', label: '外观设置' },
  { id: 'cache', label: '缓存设置' },
  { id: 'netease', label: '网易云下载全局设置' }
]

const fileInputRef = ref<HTMLInputElement | null>(null)
const triggerBgUpload = () => {
  fileInputRef.value?.click()
}

// 缩放设置
const uiScalePercent = ref(100)

const setScale = (val: number) => {
  uiScalePercent.value = val
  const scale = (val / 100).toFixed(2)
  document.documentElement.style.setProperty('--ui-scale', scale)
  localStorage.setItem('2fmusic_ui_scale', scale)
  message.success(`界面缩放比例已调整为 ${scale}`)
}

// 缓存设置
const cacheCovers = ref(false)
const cacheLyrics = ref(false)

const loadCacheSettings = () => {
  cacheCovers.value = localStorage.getItem('2fmusic_cache_covers') === 'true'
  cacheLyrics.value = localStorage.getItem('2fmusic_cache_lyrics') === 'true'
}

const saveCacheSettings = () => {
  localStorage.setItem('2fmusic_cache_covers', String(cacheCovers.value))
  localStorage.setItem('2fmusic_cache_lyrics', String(cacheLyrics.value))
  message.success('设置已自动保存并应用')
}

// 网易云 API
const neteaseApi = ref('')
const neteaseDownloadDir = ref('')

let dockerTimer: number | null = null

const loadNeteaseSettings = async () => {
  await systemStore.fetchNeteaseConfig()
  neteaseApi.value = systemStore.neteaseConfig.api_base || ''
  neteaseDownloadDir.value = systemStore.neteaseConfig.download_dir || ''
}

const saveNeteaseSettings = async () => {
  const res = await systemStore.saveNeteaseConfig(neteaseDownloadDir.value.trim(), neteaseApi.value.trim())
  if (res.success) {
    message.success('网易云下载配置已成功保存')
  } else {
    message.error(res.error)
  }
}

const disconnectNetease = async () => {
  const res = await systemStore.saveNeteaseConfig('', ' ') // 空值触发断开
  if (res.success) {
    message.success('已安全断开与当前网易云 API 服务的连接')
    neteaseApi.value = ''
  } else {
    message.error('断开失败')
  }
}

// Docker 自动部署
const handleDockerInstall = async () => {
  const res = await systemStore.installNeteaseDocker()
  if (res.success) {
    message.info('Docker 自动部署已启动，请稍候...')
    startDockerPoll()
  } else {
    message.error(res.error)
  }
}

const startDockerPoll = () => {
  if (dockerTimer) return
  dockerTimer = window.setInterval(async () => {
    await systemStore.fetchDockerInstallStatus()
    const stat = systemStore.dockerInstallStatus.status
    if (stat === 'success') {
      message.success('网易云 API 镜像服务启动成功，已自动连接！')
      cleanupDockerPoll()
      await systemStore.fetchNeteaseConfig()
      await systemStore.fetchNeteaseUserStatus()
      await systemStore.checkDockerContainer()
      loadNeteaseSettings()
    } else if (stat === 'error') {
      message.error(`部署失败: ${systemStore.dockerInstallStatus.error}`)
      cleanupDockerPoll()
      systemStore.checkDockerContainer()
    }
  }, 3000)
}

const cleanupDockerPoll = () => {
  if (dockerTimer) {
    clearInterval(dockerTimer)
    dockerTimer = null
  }
}

// 清理缓存
const handleClearCache = async () => {
  try {
    // 1. 清空所有 Cache Storage (Service Worker 缓存)
    if (window.caches) {
      const keys = await window.caches.keys()
      await Promise.all(keys.map(k => window.caches.delete(k)))
    }

    // 2. 清理 IndexedDB
    try {
      await musicDB.clearAll()
    } catch (e) {
      console.warn('清空 IndexedDB 缓存失败:', e)
    }

    // 3. 清理 localStorage 和 sessionStorage 中除了播放器关键状态以外的内容
    const keysToKeep = ['2fmusic_state', '2fmusic_favs', '2fmusic_playlist', '2fmusic_ui_scale']
    const keys = Object.keys(localStorage)
    keys.forEach(k => {
      if (!keysToKeep.includes(k)) {
        localStorage.removeItem(k)
      }
    })

    message.success('浏览器应用缓存已清空，即将自动刷新页面')
    setTimeout(() => {
      window.location.reload()
    }, 1500)
  } catch (e) {
    message.error('清除缓存失败')
  }
}

// 退出登录
const handleLogout = () => {
  message.info('正在安全注销会话...')
  setTimeout(() => {
    // 直接重定向至后端的 /logout 路由
    window.location.href = `${getBaseUrl()}/logout`
  }, 1000)
}

const cleanupResources = () => {
  cleanupDockerPoll()
}

onMounted(async () => {
  // 加载缩放
  const savedScale = localStorage.getItem('2fmusic_ui_scale')
  if (savedScale) {
    const scaleVal = parseFloat(savedScale)
    if (scaleVal <= 0.95) {
      uiScalePercent.value = 90
    } else if (scaleVal >= 1.05) {
      uiScalePercent.value = 110
    } else {
      uiScalePercent.value = 100
    }
    // 重新写回规范化的比例，保证全局一致
    const normalizedScale = (uiScalePercent.value / 100).toFixed(2)
    document.documentElement.style.setProperty('--ui-scale', normalizedScale)
    localStorage.setItem('2fmusic_ui_scale', normalizedScale)
  } else {
    uiScalePercent.value = 100
  }

  loadCacheSettings()

  // 加载网易云配置
  await loadNeteaseSettings()

  // 如果尚未配置 API，检查 Docker 并尝试自动连接
  if (!systemStore.neteaseConfig.api_base) {
    await systemStore.checkDockerContainer()

    if (systemStore.dockerContainerStatus.docker_installed &&
      systemStore.dockerContainerStatus.container_running) {
      // 直接尝试保存默认 API 地址（含连通性测试），不走 install 流程
      let connected = false
      for (let attempt = 0; attempt < 3; attempt++) {
        const res = await systemStore.saveNeteaseConfig(neteaseDownloadDir.value.trim(), 'http://localhost:23236')
        if (res.success) {
          connected = true
          neteaseApi.value = 'http://localhost:23236'
          await systemStore.checkDockerContainer()
          break
        }
        if (attempt < 2) {
          await new Promise(r => setTimeout(r, 2000))
        }
      }
      if (!connected) {
        message.warning('自动连接本地 API 服务失败，请手动配置 API 地址')
      }
    }
  }

  // 若当前正在部署 Docker，继续拉取状态
  if (systemStore.dockerInstallStatus.status === 'running') {
    startDockerPoll()
  }
})

onUnmounted(() => {
  cleanupResources()
})

// 歌词刮削来源偏好变更
const handleLyricsPrefChange = (value: 'embedded' | 'network') => {
  preferencesStore.saveLyricsPreference(value)
  message.success(`歌词刮削来源已切换为「${value === 'embedded' ? '优先内嵌' : '优先网络'}」`)
}

// 选择并上传背景图
const handleBgUpload = async (e: Event) => {
  const target = e.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  try {
    message.info('正在设置背景，请稍候...')
    await preferencesStore.setLocalBackground(file)
    message.success('已成功设置本地个性化背景')
  } catch (err: any) {
    message.error('背景设置失败: ' + (err.message || '未知错误'))
  } finally {
    target.value = ''
  }
}

// 清除背景图片
const handleClearBg = async () => {
  const success = await preferencesStore.clearBackground()
  if (success) {
    message.success('个性化背景已成功清除')
  } else {
    message.error('清除背景失败')
  }
}
</script>
