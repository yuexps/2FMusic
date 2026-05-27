import { defineStore } from 'pinia'
import { ref } from 'vue'
import { wsClient } from '../api/ws'
import { musicDB } from '../utils/indexedDB'

export const usePreferencesStore = defineStore('preferences', () => {
  const customBgEnabled = ref(false)
  const bgUrl = ref<string | null>(null)
  const bgTimestamp = ref<string>('0')
  const themeMode = ref<'system' | 'light' | 'dark'>('system')
  const lyricsSourcePref = ref<'embedded' | 'network'>('embedded')

  // 跟踪当前 blob URL 以便后续释放
  let _currentBlobUrl: string | null = null

  // 创建 Blob URL，先释放旧 URL
  const _createBlobUrl = (blob: Blob): string => {
    _revokeBlobUrl()
    const url = URL.createObjectURL(blob)
    _currentBlobUrl = url
    return url
  }

  const _revokeBlobUrl = () => {
    if (_currentBlobUrl) {
      URL.revokeObjectURL(_currentBlobUrl)
      _currentBlobUrl = null
    }
  }

  // 从 IndexedDB 读取背景 Blob 生成 URL
  const _loadLocalBlobUrl = async (): Promise<string | null> => {
    const record = await musicDB.getBackground()
    if (!record) return null
    if (record.timestamp) {
      bgTimestamp.value = record.timestamp
    }
    return _createBlobUrl(record.blob)
  }

  // 初始化偏好设置，建立连接后首先拉取
  const fetchPreferences = async () => {
    // 0. 加载本地主题配置并应用
    const localTheme = localStorage.getItem('2fmusic_theme_mode') || 'system'
    themeMode.value = localTheme as any
    applyTheme(localTheme as any)

    // 1. 读取本地缓存实现首屏极速无闪烁渲染
    const localEnabled = localStorage.getItem('2fmusic_custom_bg_enabled') === 'true'
    const localTimestamp = localStorage.getItem('2fmusic_custom_bg_timestamp') || '0'

    bgTimestamp.value = localTimestamp

    if (localEnabled) {
      bgUrl.value = await _loadLocalBlobUrl()
    } else {
      bgUrl.value = null
    }

    // 强一致性修正：有图即开，无图即关
    customBgEnabled.value = !!bgUrl.value
    localStorage.setItem('2fmusic_custom_bg_enabled', String(customBgEnabled.value))

    // 2. 从服务端加载歌词刮削偏好
    await fetchLyricsPreference()
  }

  // 拉取歌词刮削来源偏好
  const fetchLyricsPreference = async () => {
    try {
      const data = await wsClient.sendRequest('system/get_lyrics_preference')
      if (data && data.value) {
        lyricsSourcePref.value = data.value as 'embedded' | 'network'
      }
    } catch (e) {
      console.warn('Failed to fetch lyrics preference:', e)
    }
  }

  // 保存歌词刮削来源偏好
  const saveLyricsPreference = async (value: 'embedded' | 'network') => {
    lyricsSourcePref.value = value
    try {
      await wsClient.sendRequest('system/save_lyrics_preference', { value })
    } catch (e) {
      console.warn('Failed to save lyrics preference:', e)
    }
  }

  // 保存偏好设置到本地
  const savePreferences = async () => {
    localStorage.setItem('2fmusic_custom_bg_enabled', String(customBgEnabled.value))

    // 更新背景显示状态
    if (customBgEnabled.value) {
      bgUrl.value = await _loadLocalBlobUrl()
    } else {
      _revokeBlobUrl()
      bgUrl.value = null
    }
  }

  // 本地设置背景（原始 Blob）
  const setLocalBackground = async (blob: Blob) => {
    const newTimestamp = String(Date.now())
    bgTimestamp.value = newTimestamp
    await musicDB.saveBackground(blob, newTimestamp)
    localStorage.setItem('2fmusic_custom_bg_timestamp', bgTimestamp.value)
    customBgEnabled.value = true
    localStorage.setItem('2fmusic_custom_bg_enabled', 'true')

    // 立即用 blob URL 展示原图
    bgUrl.value = _createBlobUrl(blob)
    await savePreferences()
  }

  // 清除自定义背景图片
  const clearBackground = async (): Promise<boolean> => {
    try {
      await musicDB.deleteBackground()
      localStorage.setItem('2fmusic_custom_bg_enabled', 'false')
      customBgEnabled.value = false
      _revokeBlobUrl()
      bgUrl.value = null

      await savePreferences()
      return true
    } catch (e) {
      console.error('Failed to clear background:', e)
      return false
    }
  }

  // 应用主题样式
  const applyTheme = (mode: 'system' | 'light' | 'dark') => {
    const root = document.documentElement
    root.classList.remove('theme-light', 'theme-dark')
    if (mode !== 'system') {
      root.classList.add(`theme-${mode}`)
    }
  }

  // 切换主题模式
  const setThemeMode = (mode: 'system' | 'light' | 'dark') => {
    themeMode.value = mode
    localStorage.setItem('2fmusic_theme_mode', mode)
    applyTheme(mode)
  }

  return {
    customBgEnabled,
    bgUrl,
    themeMode,
    lyricsSourcePref,
    fetchPreferences,
    savePreferences,
    setLocalBackground,
    clearBackground,
    setThemeMode,
    fetchLyricsPreference,
    saveLyricsPreference
  }
})
