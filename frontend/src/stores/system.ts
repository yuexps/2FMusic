import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Song, SystemStatus, DownloadTask, NeteaseSong } from '../types'
import { wsClient } from '../api/ws'
import { musicDB } from '../utils/indexedDB'
import { coverCacheManager } from '../utils/coverCache'

export const useSystemStore = defineStore('system', () => {
  const songs = ref<Song[]>([])
  const mountPoints = ref<string[]>([])
  const status = ref<SystemStatus>({
    scanning: false,
    total: 0,
    processed: 0,
    current_file: '',
    library_version: 0,
    music_count: 0,
    playlist_count: 0
  })

  // 下载任务管理
  const downloadTasks = ref<Record<string, DownloadTask>>({})

  // 网易云 API 配置与状态
  const savedConfig = localStorage.getItem('2fmusic_netease_config')
  const neteaseConfig = ref(savedConfig ? JSON.parse(savedConfig) : {
    download_dir: '',
    api_base: '',
    max_concurrent: 5,
    quality: 'exhigh'
  })

  const savedUser = localStorage.getItem('2fmusic_netease_user')
  const neteaseUser = ref<{
    logged_in: boolean
    nickname?: string
    user_id?: number
    avatar?: string
    is_vip?: boolean
  }>(savedUser ? JSON.parse(savedUser) : {
    logged_in: false
  })

  // Docker 自动安装网易云 API 进度
  const dockerInstallStatus = ref({
    status: 'idle', // idle, running, success, error
    progress: 0,
    step: '',
    error: null as string | null
  })

  // Docker 容器存在状态（从后端实时检查）
  const dockerContainerStatus = ref({
    docker_installed: false,
    container_exists: false,
    container_running: false
  })

  // 网易云每日推荐缓存
  const savedRecommend = localStorage.getItem('2fmusic_netease_recommend')
  const neteaseRecommendSongs = ref<NeteaseSong[]>(savedRecommend ? JSON.parse(savedRecommend) : [])

  // 1. WebSocket 连接与数据广播订阅
  const initWebSocket = () => {
    wsClient.connect()

    // 订阅连接成功事件
    wsClient.subscribe('open', () => {
      console.log('WS Open: Fetching initial library state...')
      fetchSystemStatus()
      fetchSongs()
      fetchMountPoints()
      fetchNeteaseConfig()
      fetchNeteaseUserStatus()
    })

    // 订阅库扫描状态
    wsClient.subscribe('scan_status', (data: any) => {
      console.log('WS: Received scan status:', data)
      const oldVersion = status.value.library_version
      status.value = {
        ...status.value,
        ...data
      }
      if (oldVersion && !status.value.library_version) {
        status.value.library_version = oldVersion
      }
    })

    // 订阅库文件变更通知
    wsClient.subscribe('library_changed', (data: any) => {
      console.log('WS: Received library changed:', data)
      if (data && data.library_version) {
        status.value.library_version = data.library_version
      }
      fetchSongs()
      fetchSystemStatus() // 触发状态更新以获取最新音乐数量统计
    })

    // 订阅下载任务实时进度
    wsClient.subscribe('download_status', (data: any) => {
      console.log('WS: Received download task status:', data)
      if (data && data.task_id) {
        downloadTasks.value[data.task_id] = data as DownloadTask
      }
    })
  }

  // 2. 本地音乐库接口
  const fetchSongs = async () => {
    try {
      const data = await wsClient.sendRequest('music/get_list')
      songs.value = data
      // 同步缓存
      localStorage.setItem('2fmusic_playlist', JSON.stringify(songs.value))
    } catch (e) {
      console.error('Failed to fetch songs via WS:', e)
    }
  }

  const deleteSong = async (songId: string) => {
    try {
      await wsClient.sendRequest('music/delete', { song_id: songId })
      songs.value = songs.value.filter(s => s.id !== songId)
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '删除异常' }
    }
  }

  const clearMetadata = async (songId: string) => {
    try {
      // 1. 请求服务端清除元数据缓存。服务端会清空封面与歌词物理文件、置空数据库状态，并广播 library_changed
      await wsClient.sendRequest('music/clear_metadata', { song_id: songId })

      // 2. 清除前端本地数据库 IndexedDB 中的对应缓存
      await musicDB.deleteCover(songId).catch(err => console.warn('Failed to delete cover in IndexedDB:', err))
      await musicDB.deleteLyrics(songId).catch(err => console.warn('Failed to delete lyrics in IndexedDB:', err))

      // 3. 清除前端内存缓存池中的 Object URL，防止继续使用旧的 Blob URL
      coverCacheManager.delete(songId)

      // 4. 重新请求封面和歌词，触发后端重新刮削(索引)
      const targetSong = songs.value.find(s => s.id === songId)
      if (targetSong) {
        const payload = {
          song_id: songId,
          title: targetSong.title,
          artist: targetSong.artist,
          filename: targetSong.filename
        }
        wsClient.sendRequest('music/album-art', payload).catch(e => console.warn('Re-fetch album art failed:', e))
        wsClient.sendRequest('music/lyrics', payload).catch(e => console.warn('Re-fetch lyrics failed:', e))
      }

      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '添加异常' }
    }
  }

  // 3. 目录管理接口
  const fetchMountPoints = async () => {
    try {
      const data = await wsClient.sendRequest('mount/list')
      mountPoints.value = data
    } catch (e) {
      console.error('Failed to fetch mount points via WS:', e)
    }
  }

  const addMountPoint = async (path: string) => {
    try {
      await wsClient.sendRequest('mount/add', { path })
      await fetchMountPoints()
      return { success: true, message: '挂载点添加成功' }
    } catch (e: any) {
      return { success: false, error: e.message || '添加异常' }
    }
  }

  const removeMountPoint = async (path: string) => {
    try {
      await wsClient.sendRequest('mount/delete', { path })
      await fetchMountPoints()
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '移除异常' }
    }
  }

  const triggerScan = async (path: string) => {
    try {
      const message = await wsClient.sendRequest('mount/scan', { path })
      return { success: true, message }
    } catch (e: any) {
      return { success: false, error: e.message || '扫描触发失败' }
    }
  }

  const triggerRescrape = async (path: string) => {
    try {
      const message = await wsClient.sendRequest('mount/retry_scrape', { path })
      return { success: true, message }
    } catch (e: any) {
      return { success: false, error: e.message || '重试刮削触发失败' }
    }
  }

  // 4. 系统运行状态接口
  const fetchSystemStatus = async () => {
    try {
      const data = await wsClient.sendRequest('system/get_status')
      status.value = data
    } catch (e) {
      console.error('Failed to fetch system status via WS:', e)
    }
  }

  // 5. 网易云 API 客户端对接接口
  const fetchNeteaseConfig = async () => {
    try {
      const data = await wsClient.sendRequest('netease/get_config')
      neteaseConfig.value = data
      localStorage.setItem('2fmusic_netease_config', JSON.stringify(data))
    } catch (e) {
      console.error('Failed to fetch netease config via WS:', e)
    }
  }

  const saveNeteaseConfig = async (downloadDir: string, apiBase: string) => {
    try {
      const data = await wsClient.sendRequest('netease/save_config', {
        download_dir: downloadDir,
        api_base: apiBase
      })
      neteaseConfig.value = data
      localStorage.setItem('2fmusic_netease_config', JSON.stringify(data))
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '保存异常' }
    }
  }

  const fetchNeteaseUserStatus = async () => {
    try {
      const data = await wsClient.sendRequest('netease/login_status')
      neteaseUser.value = data
      localStorage.setItem('2fmusic_netease_user', JSON.stringify(data))
    } catch (e) {
      neteaseUser.value = { logged_in: false }
      localStorage.removeItem('2fmusic_netease_user')
    }
  }

  const logoutNetease = async () => {
    try {
      await wsClient.sendRequest('netease/logout')
      neteaseUser.value = { logged_in: false }
      localStorage.removeItem('2fmusic_netease_user')
      clearNeteaseRecommendCache() // 退出登录时清空每日推荐列表缓存
      return { success: true }
    } catch (e) {
      console.error('Failed to logout netease via WS:', e)
      return { success: false }
    }
  }

  const fetchNeteaseRecommendSongs = async (force = false) => {
    if (neteaseRecommendSongs.value.length > 0 && !force) {
      return neteaseRecommendSongs.value
    }
    try {
      const data = await wsClient.sendRequest('netease/recommend')
      neteaseRecommendSongs.value = data || []
      localStorage.setItem('2fmusic_netease_recommend', JSON.stringify(neteaseRecommendSongs.value))
      return neteaseRecommendSongs.value
    } catch (e) {
      console.error('Failed to fetch netease recommend songs via WS:', e)
      throw e
    }
  }

  const clearNeteaseRecommendCache = () => {
    neteaseRecommendSongs.value = []
    localStorage.removeItem('2fmusic_netease_recommend')
  }

  const startNeteaseDownload = async (song: NeteaseSong, targetDir?: string) => {
    try {
      const payload = {
        id: song.id,
        title: song.title,
        artist: song.artist,
        album: song.album,
        cover: song.cover,
        level: neteaseConfig.value.quality || 'exhigh',
        target_dir: targetDir || undefined
      }
      const data = await wsClient.sendRequest('netease/download', payload)
      const taskId = data.task_id
      downloadTasks.value[taskId] = {
        task_id: taskId,
        status: 'pending',
        progress: 0,
        title: song.title,
        artist: song.artist
      }
      return { success: true, task_id: taskId }
    } catch (e: any) {
      return { success: false, error: e.message || '启动下载异常' }
    }
  }

  const installNeteaseDocker = async () => {
    try {
      await wsClient.sendRequest('netease/install_service')
      dockerInstallStatus.value.status = 'running'
      dockerInstallStatus.value.step = '安装任务已提交'
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '安装触发异常' }
    }
  }

  const fetchDockerInstallStatus = async () => {
    try {
      const data = await wsClient.sendRequest('netease/install_status')
      dockerInstallStatus.value = data
    } catch (e) {
      console.error('Failed to fetch docker install status via WS:', e)
    }
  }

  const checkDockerContainer = async () => {
    try {
      const data = await wsClient.sendRequest('netease/check_container')
      dockerContainerStatus.value = data
    } catch (e) {
      console.error('Failed to check docker container via WS:', e)
    }
  }

  return {
    songs,
    mountPoints,
    status,
    downloadTasks,
    neteaseConfig,
    neteaseUser,
    dockerInstallStatus,
    dockerContainerStatus,
    neteaseRecommendSongs,
    initWebSocket,
    fetchSongs,
    deleteSong,
    clearMetadata,
    fetchMountPoints,
    addMountPoint,
    removeMountPoint,
    triggerScan,
    triggerRescrape,
    fetchSystemStatus,
    fetchNeteaseConfig,
    saveNeteaseConfig,
    fetchNeteaseUserStatus,
    logoutNetease,
    fetchNeteaseRecommendSongs,
    clearNeteaseRecommendCache,
    startNeteaseDownload,
    installNeteaseDocker,
    fetchDockerInstallStatus,
    checkDockerContainer
  }
})
