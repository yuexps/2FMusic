import { defineStore } from 'pinia'
import { ref } from 'vue'
import { wsClient } from '../api/ws'
import { getApiUrl } from '../utils/path'
import { musicDB } from '../utils/indexedDB'

export const usePreferencesStore = defineStore('preferences', () => {
  const customBgEnabled = ref(false)
  const customBgSync = ref(false)
  const bgUrl = ref<string | null>(null)
  const bgTimestamp = ref<string>('0')
  const themeMode = ref<'system' | 'light' | 'dark'>('system')

  // 跟踪当前 blob URL 以便后续释放
  let _currentBlobUrl: string | null = null

  // 从 Blob 创建 blob URL，同时释放之前的
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

  // 云端背景 URL，使用已持久化的时间戳实现缓存复用
  const _cloudBgUrl = () => getApiUrl(`/api/music/backgrounds/cloud_bg.webp?t=${bgTimestamp.value}`)

  // 后台静默下载云端新图，保存到 IndexedDB 后无缝切换（不闪白）
  const _silentDownloadCloudBg = async (timestamp: string) => {
    try {
      const resp = await fetch(_cloudBgUrl())
      if (!resp.ok) return
      const blob = await resp.blob()
      await musicDB.saveBackground(blob, timestamp)
      _revokeBlobUrl()
      bgUrl.value = _createBlobUrl(blob)
    } catch (e) {
      console.warn('Silent cloud background download failed, keeping local:', e)
    }
  }

  // 从 IndexedDB 读取背景 Blob 并生成 blob URL（同时恢复时间戳）
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

    // 1. 优先读取本地缓存实现首屏极速无闪烁渲染
    const localEnabled = localStorage.getItem('2fmusic_custom_bg_enabled') === 'true'
    const localSync = localStorage.getItem('2fmusic_custom_bg_sync') === 'true'
    const localTimestamp = localStorage.getItem('2fmusic_custom_bg_timestamp') || '0'

    customBgSync.value = localSync
    bgTimestamp.value = localTimestamp

    if (localEnabled) {
      if (localSync) {
        // 云同步模式：优先读本地 IndexedDB，零网络；无本地数据时走云端 URL
        bgUrl.value = (await _loadLocalBlobUrl()) || _cloudBgUrl()
      } else {
        bgUrl.value = await _loadLocalBlobUrl()
      }
    } else {
      bgUrl.value = null
    }

    // 强一致性修正：有图即开，无图即关
    customBgEnabled.value = !!bgUrl.value
    localStorage.setItem('2fmusic_custom_bg_enabled', String(customBgEnabled.value))

    // 2. 发起 WebSocket 请求向服务端同步最权威的偏好数据
    try {
      const data = await wsClient.sendRequest('system/get_preferences')
      if (data) {
        customBgEnabled.value = !!data.custom_bg_enabled

        // 比较服务器时间戳是否有变化
        const serverTimestamp = String(data.custom_bg_timestamp || '0')

        if (customBgEnabled.value) {
          if (customBgSync.value) {
            if (serverTimestamp !== bgTimestamp.value) {
              // 时间戳变化 → 保持本地图片不变，后台静默下载云端新图
              bgTimestamp.value = serverTimestamp
              _silentDownloadCloudBg(serverTimestamp)
            } else {
              // 时间戳一致 → 本地 IndexedDB 的图就是最新的，零网络
              const localUrl = await _loadLocalBlobUrl()
              if (localUrl) {
                bgUrl.value = localUrl
              }
            }
          } else {
            bgUrl.value = (await _loadLocalBlobUrl()) || null
          }
        } else {
          _revokeBlobUrl()
          bgUrl.value = null
        }

        // 强一致性修正：有图即开，无图即关
        customBgEnabled.value = !!bgUrl.value

        // 重新写回 LocalStorage
        localStorage.setItem('2fmusic_custom_bg_enabled', String(customBgEnabled.value))
        localStorage.setItem('2fmusic_custom_bg_sync', String(customBgSync.value))
        localStorage.setItem('2fmusic_custom_bg_timestamp', bgTimestamp.value)
      }
    } catch (e) {
      console.warn('Failed to sync preferences via WS:', e)
    }
  }

  // 保存偏好开关设置到云端
  const savePreferences = async () => {
    localStorage.setItem('2fmusic_custom_bg_enabled', String(customBgEnabled.value))
    localStorage.setItem('2fmusic_custom_bg_sync', String(customBgSync.value))

    await wsClient.sendRequest('system/save_preferences', {
      prefs: {
        custom_bg_enabled: customBgEnabled.value,
        custom_bg_timestamp: bgTimestamp.value
      }
    })

    // 更新状态
    if (customBgEnabled.value) {
      if (customBgSync.value) {
        // 尝试自动将本地图片上传到云端
        const record = await musicDB.getBackground()
        if (record && record.blob) {
          const newTimestamp = String(Date.now())
          const formData = new FormData()
          formData.append('file', record.blob)
          formData.append('timestamp', newTimestamp)
          try {
            const resp = await fetch(getApiUrl('/api/music/background/upload'), { method: 'POST', body: formData })
            const res = await resp.json()
            if (res.success) {
              bgTimestamp.value = newTimestamp
              localStorage.setItem('2fmusic_custom_bg_timestamp', bgTimestamp.value)
              await musicDB.updateBackgroundTimestamp(bgTimestamp.value)
              _revokeBlobUrl()
              bgUrl.value = _cloudBgUrl()
              return
            }
          } catch (e) {
            console.warn('Auto-upload local bg failed, keep local display:', e)
          }
        }
        // 上传失败或无本地数据 → 回退到本地显示
        bgUrl.value = await _loadLocalBlobUrl()
      } else {
        bgUrl.value = await _loadLocalBlobUrl()
      }
    } else {
      _revokeBlobUrl()
      bgUrl.value = null
    }
  }

  // 纯本地设置背景（原始 Blob，前端不做压缩）
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

  // 上传背景至云端 (使用 HTTP 接口传输文件)
  const uploadCloudBackground = async (file: File): Promise<{ success: boolean; error?: string }> => {
    const formData = new FormData()
    formData.append('file', file)

    // 用前端的上传完成时间作为这个背景图版本的唯一 ID
    const newTimestamp = String(Date.now())
    formData.append('timestamp', newTimestamp)

    try {
      const resp = await fetch(getApiUrl('/api/music/background/upload'), {
        method: 'POST',
        body: formData
      })
      const res = await resp.json()
      if (res.success) {
        customBgEnabled.value = true
        customBgSync.value = true

        localStorage.setItem('2fmusic_custom_bg_enabled', 'true')
        localStorage.setItem('2fmusic_custom_bg_sync', 'true')

        _revokeBlobUrl()
        bgTimestamp.value = newTimestamp
        localStorage.setItem('2fmusic_custom_bg_timestamp', bgTimestamp.value)
        await musicDB.updateBackgroundTimestamp(bgTimestamp.value)
        bgUrl.value = _cloudBgUrl()

        // 通知后端更新偏好字段
        await savePreferences()
        return { success: true }
      } else {
        return { success: false, error: res.error || '上传处理失败' }
      }
    } catch (e: any) {
      console.error('Failed to upload cloud background:', e)
      return { success: false, error: e.message || '网络连接异常' }
    }
  }

  // 清除自定义背景图片
  const clearBackground = async (): Promise<boolean> => {
    try {
      await musicDB.deleteBackground()
      localStorage.setItem('2fmusic_custom_bg_enabled', 'false')
      customBgEnabled.value = false
      _revokeBlobUrl()
      bgUrl.value = null

      if (customBgSync.value) {
        await fetch(getApiUrl('/api/music/background/delete'), { method: 'POST' })
      }

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

  // 切换主题模式并持久化
  const setThemeMode = (mode: 'system' | 'light' | 'dark') => {
    themeMode.value = mode
    localStorage.setItem('2fmusic_theme_mode', mode)
    applyTheme(mode)
  }

  return {
    customBgEnabled,
    customBgSync,
    bgUrl,
    themeMode,
    fetchPreferences,
    savePreferences,
    setLocalBackground,
    uploadCloudBackground,
    clearBackground,
    setThemeMode
  }
})
