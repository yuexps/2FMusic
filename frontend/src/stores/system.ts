import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Song, SystemStatus, DownloadTask, NeteaseSong } from '../types'
import { wsClient } from '../api/ws'
import { musicDB } from '../utils/indexedDB'
import { coverCacheManager } from '../utils/coverCache'
import { usePlayerStore } from './player'

export const useSystemStore = defineStore('system', () => {
  let parsedPlaylist: Song[] = []
  try {
    const savedPlaylist = localStorage.getItem('2fmusic_playlist')
    if (savedPlaylist) {
      parsedPlaylist = JSON.parse(savedPlaylist)
    }
  } catch (e) {
    console.warn('解析缓存播放列表失败：', e)
  }
  const songs = ref<Song[]>(parsedPlaylist)
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

  // 下载任务
  const downloadTasks = ref<Record<string, DownloadTask>>({})

  // 网易云配置与状态
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

  const dockerContainerStatus = ref({
    docker_installed: false,
    container_exists: false,
    container_running: false
  })

  // 网易云每日推荐缓存
  const savedRecommend = localStorage.getItem('2fmusic_netease_recommend')
  const neteaseRecommendSongs = ref<NeteaseSong[]>(savedRecommend ? JSON.parse(savedRecommend) : [])

  let isWsInitialized = false

  // 1. WebSocket 连接与数据广播订阅
  const initWebSocket = () => {
    wsClient.connect()
    if (isWsInitialized) return
    isWsInitialized = true

    // 连接成功时获取初始状态
    wsClient.subscribe('open', () => {
      console.log('WebSocket 连接开启：正在拉取初始音乐库状态...')
      fetchSystemStatus()
      fetchSongs()
      fetchMountPoints()
      fetchNeteaseConfig()
      fetchNeteaseUserStatus()
    })

    // 库扫描状态
    wsClient.subscribe('scan_status', (data: any) => {
      console.log('WebSocket：收到扫描状态更新:', data)
      const oldVersion = status.value.library_version
      status.value = {
        ...status.value,
        ...data
      }
      if (oldVersion && !status.value.library_version) {
        status.value.library_version = oldVersion
      }
    })

    // 库变更通知处理
    wsClient.subscribe('library_changed', async (data: any) => {
      console.log('WebSocket：收到音乐库变更通知:', data)
      if (!data) return

      const { event_type, song_ids = [], fields = [], timestamp = Date.now() } = data
      const playerStore = usePlayerStore()

      // 1. 定向擦除本地 IndexedDB 缓存与 ObjectURL
      for (const id of song_ids) {
        if (fields.includes('cover')) {
          await musicDB.deleteCover(id)
          coverCacheManager.delete(id)
        }
        if (fields.includes('lyrics')) {
          await musicDB.deleteLyrics(id)
        }
      }

      // 2. 当前播放曲目定向热重载
      const currentSong = playerStore.currentSong
      if (currentSong && song_ids.includes(currentSong.id)) {
        if (fields.includes('lyrics')) {
          playerStore.reloadCurrentLyrics()
        }
        if (fields.includes('cover')) {
          if (currentSong.album_art) {
            const baseUrl = currentSong.album_art.split('?')[0]
            currentSong.album_art = `${baseUrl}?v=${timestamp}`
          }
        }
      }

      // 3. 按 event_type 局部增量刷新或重新拉取列表
      if (event_type === 'update' && song_ids.length > 0) {
        songs.value = songs.value.map(song => {
          if (song_ids.includes(song.id)) {
            const updated = { ...song }
            if (fields.includes('cover')) {
              updated.has_cover = true
              const baseUrl = (updated.album_art || `/api/music/covers/${song.id}.webp`).split('?')[0]
              updated.album_art = `${baseUrl}?v=${timestamp}`
            }
            if (fields.includes('lyrics')) {
              updated.has_lyrics = true
            }
            return updated
          }
          return song
        })
      } else {
        fetchSongs()
      }

      fetchSystemStatus()
    })

    // 下载任务进度
    wsClient.subscribe('download_status', (data: any) => {
      console.log('WebSocket：收到下载任务状态更新:', data)
      if (data && data.task_id) {
        if (data.status === 'deleted') {
          delete downloadTasks.value[data.task_id]
        } else {
          downloadTasks.value[data.task_id] = data as DownloadTask
        }
      }
    })
  }

  // 本地音乐库接口
  const fetchSongs = async () => {
    try {
      const data = await wsClient.sendRequest('music/get_list')
      songs.value = Array.isArray(data) ? data : []
      // 同步缓存
      localStorage.setItem('2fmusic_playlist', JSON.stringify(songs.value))

      // 触发播放器与播放列表的失效歌曲清洗
      const playerStore = usePlayerStore()
      playerStore.cleanInvalidSongs(songs.value)
    } catch (e: any) {
      if (!e?.isWSClosed) {
        console.error('通过 WebSocket 获取歌曲列表失败:', e)
      }
    }
  }

  const clearUserData = () => {
    songs.value = []
    mountPoints.value = []
    status.value = {
      scanning: false,
      total: 0,
      processed: 0,
      current_file: '',
      library_version: 0,
      music_count: 0,
      playlist_count: 0
    }
    localStorage.removeItem('2fmusic_playlist')

    const playerStore = usePlayerStore()
    playerStore.stop()
  }

  // 安全注销用户会话凭据
  const logout = () => {
    localStorage.removeItem('2fmusic_password')
    window.dispatchEvent(new CustomEvent('2fmusic-unauthorized'))
  }

  const deleteSong = async (songId: string) => {
    try {
      await wsClient.sendRequest('music/delete', { song_id: songId })
      songs.value = songs.value.filter(s => String(s.id) !== String(songId))
      
      const playerStore = usePlayerStore()
      playerStore.cleanInvalidSongs(songs.value)

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
      await musicDB.deleteCover(songId).catch(err => console.warn('从 IndexedDB 删除封面失败:', err))
      await musicDB.deleteLyrics(songId).catch(err => console.warn('从 IndexedDB 删除歌词失败:', err))

      // 3. 清除前端内存缓存池中的 Object URL，防止继续使用旧的 Blob URL
      coverCacheManager.delete(songId)

      // 4. 重新请求封面和歌词，触发后端重新刮削(索引)
      const targetSong = songs.value.find(s => s.id === songId)
      if (targetSong) {
        const payload = {
          song_id: songId,
          title: targetSong.title,
          artist: targetSong.artist,
          album: targetSong.album,
          filename: targetSong.filename
        }
        wsClient.sendRequest('music/album-art', payload).catch(e => console.warn('重新获取专辑封面失败:', e))
        wsClient.sendRequest('music/lyrics', payload).catch(e => console.warn('重新获取歌词失败:', e))
      }

      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '添加异常' }
    }
  }

  // 目录管理接口
  const fetchMountPoints = async () => {
    try {
      const data = await wsClient.sendRequest('mount/list')
      mountPoints.value = Array.isArray(data) ? data : []
    } catch (e: any) {
      if (!e?.isWSClosed) {
        console.error('通过 WebSocket 获取目录挂载点失败:', e)
      }
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

  // 系统状态接口
  const fetchSystemStatus = async () => {
    try {
      const data = await wsClient.sendRequest('system/get_status')
      status.value = data
    } catch (e: any) {
      if (!e?.isWSClosed) {
        console.error('通过 WebSocket 获取系统状态失败:', e)
      }
    }
  }

  // 网易云 API 接口
  const fetchNeteaseConfig = async () => {
    try {
      const data = await wsClient.sendRequest('netease/get_config')
      neteaseConfig.value = data
      localStorage.setItem('2fmusic_netease_config', JSON.stringify(data))
    } catch (e: any) {
      if (!e?.isWSClosed) {
        console.error('通过 WebSocket 获取网易云配置失败:', e)
      }
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
      console.error('通过 WebSocket 登出网易云失败:', e)
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
      console.error('通过 WebSocket 获取网易云每日推荐歌曲失败:', e)
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

  const clearNeteaseDownloadTask = async (taskId: string) => {
    try {
      await wsClient.sendRequest('netease/clear_task', { task_id: taskId })
      delete downloadTasks.value[taskId]
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '清理异常' }
    }
  }

  const clearAllNeteaseDownloadTasks = async () => {
    try {
      await wsClient.sendRequest('netease/clear_all_tasks')
      for (const [tid, task] of Object.entries(downloadTasks.value)) {
        if (task.status === 'success' || task.status === 'error') {
          delete downloadTasks.value[tid]
        }
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '清理异常' }
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
      console.error('通过 WebSocket 获取 Docker 安装状态失败:', e)
    }
  }

  const checkDockerContainer = async () => {
    try {
      const data = await wsClient.sendRequest('netease/check_container')
      dockerContainerStatus.value = data
    } catch (e) {
      console.error('通过 WebSocket 检查 Docker 容器失败:', e)
    }
  }

  // 登录解锁浮层弹窗状态
  const isAuthModalOpen = ref(false)

  return {
    isAuthModalOpen,
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
    clearUserData,
    logout,
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
    clearNeteaseDownloadTask,
    clearAllNeteaseDownloadTasks,
    installNeteaseDocker,
    fetchDockerInstallStatus,
    checkDockerContainer
  }
})
