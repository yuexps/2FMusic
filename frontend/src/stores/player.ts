import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Song } from '../types'
import { wsClient } from '../api/ws'
import { musicDB } from '../utils/indexedDB'
import { getApiUrl } from '../utils/path'
import { useHistoryStore } from './history'

import { coverCacheManager } from '../utils/coverCache'

// 跟踪当前播放歌曲封面的 retain 状态
let lastRetainedSongId: string | null = null
let lastRetainedSongUrl: string | null = null

function releaseSongCoverRetain() {
  if (lastRetainedSongId && lastRetainedSongUrl) {
    coverCacheManager.release(lastRetainedSongId, lastRetainedSongUrl)
    lastRetainedSongId = null
    lastRetainedSongUrl = null
  }
}

// 加载歌曲封面（本地缓存 or 后台抓取）
const loadSongCover = async (song: Song): Promise<string> => {
  const cacheEnabled = localStorage.getItem('2fmusic_cache_covers') === 'true'
  if (!cacheEnabled) {
    return getApiUrl(song.album_art || '/ICON.PNG')
  }

  try {
    const url = await coverCacheManager.getOrCreateUrl(song.id)
    if (url) {
      if (lastRetainedSongId === song.id && lastRetainedSongUrl === url) {
        return url
      }

      // 释放之前持有的上一首歌曲封面
      releaseSongCoverRetain()

      // 登记并持有当前新歌曲的封面
      lastRetainedSongId = song.id
      lastRetainedSongUrl = url
      coverCacheManager.retain(song.id, url)

      return url
    }

    const originalUrl = song.album_art
    if (originalUrl && !originalUrl.startsWith('blob:') && !originalUrl.startsWith('data:')) {
      fetch(getApiUrl(originalUrl))
        .then(response => {
          if (!response.ok) throw new Error('Cover fetch failed')
          return response.blob()
        })
        .then(blob => {
          musicDB.saveCover(song.id, blob)
        })
        .catch(err => console.debug('缓存封面数据块失败:', err))
    }
  } catch (e) {
    console.warn('从 IndexedDB 加载歌曲封面失败:', e)
  }

  return getApiUrl(song.album_art || '/ICON.PNG')
}

export const usePlayerStore = defineStore('player', () => {
  const playlist = ref<Song[]>([])
  const queue = ref<Song[]>([])
  const currentSong = ref<Song | null>(null)
  const isPlaying = ref(false)
  const currentTime = ref(0)
  const duration = ref(0)
  const volume = ref(1.0)
  const playMode = ref<'list' | 'single' | 'random'>('list')

  let audio: HTMLAudioElement | null = null

  // 初始化播放器
  const init = () => {
    if (audio) return
    audio = new Audio()

    // 恢复 localStorage 状态
    try {
      const saved = localStorage.getItem('2fmusic_state')
      if (saved) {
        const state = JSON.parse(saved)
        if (state.volume !== undefined) {
          volume.value = state.volume
          audio.volume = state.volume
        }
        if (state.playMode) {
          playMode.value = state.playMode
        }
        if (state.currentSong) {
          currentSong.value = { ...state.currentSong }
          audio.src = getApiUrl(`/api/music/play/${state.currentSong.id}`)
          
          loadSongCover(state.currentSong).then(artUrl => {
            if (currentSong.value && currentSong.value.id === state.currentSong.id) {
              currentSong.value.album_art = artUrl
              updateMediaSession()
            }
          })

          if (!state.currentSong.album_art) {
            fetchAlbumArt(state.currentSong)
          } else {
            updateMediaSession()
          }
        }
      }
    } catch (e) {
      console.error('恢复播放器状态失败:', e)
    }

    // 绑定播放器事件
    audio.addEventListener('play', () => {
      isPlaying.value = true
      if ('mediaSession' in navigator) {
        navigator.mediaSession.playbackState = 'playing'
      }
    })

    audio.addEventListener('pause', () => {
      isPlaying.value = false
      if ('mediaSession' in navigator) {
        navigator.mediaSession.playbackState = 'paused'
      }
    })

    audio.addEventListener('timeupdate', () => {
      if (audio) {
        currentTime.value = audio.currentTime
        updateMediaPlaybackPosition()
      }
    })

    audio.addEventListener('durationchange', () => {
      if (audio) {
        duration.value = audio.duration || 0
        updateMediaPlaybackPosition()
      }
    })

    audio.addEventListener('ended', () => {
      next()
    })
  }

  const saveState = () => {
    const state = {
      volume: volume.value,
      playMode: playMode.value,
      currentSong: currentSong.value
    }
    localStorage.setItem('2fmusic_state', JSON.stringify(state))
  }

  const togglePlay = () => {
    if (!audio) init()
    if (!audio) return

    if (isPlaying.value) {
      audio.pause()
    } else {
      if (!audio.src && currentSong.value) {
        audio.src = getApiUrl(`/api/music/play/${currentSong.value.id}`)
      }
      if (audio.src) {
        audio.play().catch((err) => console.error('播放音频失败:', err))
      }
    }
  }

  const playSong = (song: Song, list?: Song[]) => {
    if (!audio) init()
    if (!audio) return

    if (list) {
      playlist.value = list
    }

    const isNewSong = !currentSong.value || currentSong.value.id !== song.id
    if (isNewSong) {
      currentSong.value = { ...song }
      audio.src = getApiUrl(`/api/music/play/${song.id}`)
      recordPlayHistory(song)
      
      // 异步加载本地封面并刷新 MediaSession
      loadSongCover(song).then(artUrl => {
        if (currentSong.value && currentSong.value.id === song.id) {
          currentSong.value.album_art = artUrl
          updateMediaSession()
        }
      })

      if (!song.album_art) {
        fetchAlbumArt(song)
      } else {
        updateMediaSession()
      }
    }

    audio.play().catch((err) => console.error('播放音频失败:', err))
    isPlaying.value = true
    saveState()
  }

  const setVolume = (val: number) => {
    volume.value = Math.max(0, Math.min(1, val))
    if (audio) {
      audio.volume = volume.value
    }
    saveState()
  }

  const seek = (time: number) => {
    if (audio) {
      audio.currentTime = time
      currentTime.value = time
    }
  }

  const next = () => {
    if (queue.value.length > 0) {
      const nextSong = queue.value.shift()!
      playSong(nextSong)
      return
    }

    if (playlist.value.length === 0) return

    if (playMode.value === 'single' && currentSong.value) {
      seek(0)
      audio?.play().catch(e => console.error('播放音频错误:', e))
      return
    }

    let nextIndex = 0
    if (playMode.value === 'random') {
      nextIndex = Math.floor(Math.random() * playlist.value.length)
    } else if (currentSong.value) {
      const currIdx = playlist.value.findIndex(s => s.id === currentSong.value!.id)
      nextIndex = (currIdx + 1) % playlist.value.length
    }

    const nextSong = playlist.value[nextIndex]
    if (nextSong) {
      playSong(nextSong)
    }
  }

  const prev = () => {
    if (playlist.value.length === 0) return

    let prevIndex = 0
    if (playMode.value === 'random') {
      prevIndex = Math.floor(Math.random() * playlist.value.length)
    } else if (currentSong.value) {
      const currIdx = playlist.value.findIndex(s => s.id === currentSong.value!.id)
      prevIndex = (currIdx - 1 + playlist.value.length) % playlist.value.length
    }

    const prevSong = playlist.value[prevIndex]
    if (prevSong) {
      playSong(prevSong)
    }
  }

  const addToQueue = (song: Song) => {
    if (queue.value.some(s => s.id === song.id)) return
    queue.value.push(song)
  }

  const removeFromQueue = (songId: string) => {
    queue.value = queue.value.filter(s => s.id !== songId)
  }

  const clearQueue = () => {
    queue.value = []
  }

  const recordPlayHistory = (song: Song) => {
    try {
      const historyStore = useHistoryStore()
      historyStore.addHistory(song.id)
    } catch (e) {
      console.error('同步播放历史至后端失败:', e)
    }
  }

  const currentLyric = ref('')

  const updateLyric = (lyricText: string) => {
    currentLyric.value = lyricText
    updateMediaSession()
  }

  // 更新 MediaSession 元数据
  const updateMediaSession = () => {
    if (!('mediaSession' in navigator) || !currentSong.value) return

    const song = currentSong.value
    const displayTitle = currentLyric.value 
      ? `${song.title} - ${song.artist}` 
      : song.title
    const displayArtist = currentLyric.value 
      ? currentLyric.value 
      : song.artist

    navigator.mediaSession.metadata = new MediaMetadata({
      title: displayTitle,
      artist: displayArtist,
      album: song.album || '',
      artwork: [
        {
          src: getApiUrl(song.album_art || '/ICON.PNG'),
          sizes: '256x256',
          type: 'image/png'
        }
      ]
    })

    navigator.mediaSession.setActionHandler('play', () => {
      togglePlay()
    })
    navigator.mediaSession.setActionHandler('pause', () => {
      togglePlay()
    })
    navigator.mediaSession.setActionHandler('previoustrack', () => {
      prev()
    })
    navigator.mediaSession.setActionHandler('nexttrack', () => {
      next()
    })
    navigator.mediaSession.setActionHandler('seekto', (details) => {
      if (details.seekTime !== undefined) {
        seek(details.seekTime)
      }
    })
  }

  const updateMediaPlaybackPosition = () => {
    if (!('mediaSession' in navigator) || !audio) return
    try {
      navigator.mediaSession.setPositionState({
        duration: audio.duration || 0,
        playbackRate: audio.playbackRate || 1.0,
        position: audio.currentTime || 0
      })
    } catch (e) {
      console.debug('媒体会话设置播放进度状态失败:', e)
    }
  }

  // 请求专辑封面
  const fetchAlbumArt = async (song: Song) => {
    try {
      const data = await wsClient.sendRequest('music/album-art', {
        title: song.title,
        artist: song.artist,
        filename: song.filename,
        song_id: song.id
      })
      if (data && data.album_art) {
        const artUrl = data.album_art
        
        // 缓存到 IndexedDB
        const cacheEnabled = localStorage.getItem('2fmusic_cache_covers') === 'true'
        if (cacheEnabled && artUrl && !artUrl.startsWith('blob:') && !artUrl.startsWith('data:')) {
          fetch(getApiUrl(artUrl))
            .then(res => res.blob())
            .then(blob => musicDB.saveCover(song.id, blob))
            .catch(err => console.debug('为缓存获取专辑封面失败:', err))
        }

        if (currentSong.value && currentSong.value.id === song.id) {
          if (cacheEnabled) {
            loadSongCover({ ...song, album_art: artUrl }).then(cachedUrl => {
              if (currentSong.value && currentSong.value.id === song.id) {
                currentSong.value.album_art = cachedUrl
                updateMediaSession()
              }
            })
          } else {
            currentSong.value.album_art = artUrl
            updateMediaSession()
          }
        }
        playlist.value = playlist.value.map(s => {
          if (s.id === song.id) {
            return { ...s, album_art: artUrl }
          }
          return s
        })
        queue.value = queue.value.map(s => {
          if (s.id === song.id) {
            return { ...s, album_art: artUrl }
          }
          return s
        })
        saveState()
      }
    } catch (e) {
      console.warn('通过 WS 获取专辑封面失败:', e)
    }
  }

  return {
    playlist,
    queue,
    currentSong,
    isPlaying,
    currentTime,
    duration,
    volume,
    playMode,
    init,
    togglePlay,
    playSong,
    setVolume,
    seek,
    next,
    prev,
    addToQueue,
    removeFromQueue,
    clearQueue,
    fetchAlbumArt,
    currentLyric,
    updateLyric
  }
})
