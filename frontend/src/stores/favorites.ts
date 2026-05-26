import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { FavoritePlaylist } from '../types'
import { wsClient } from '../api/ws'

export const useFavoritesStore = defineStore('favorites', () => {
  const playlists = ref<FavoritePlaylist[]>([])
  const favoriteSongIds = ref<string[]>([])
  const currentPlaylistId = ref<string>('default')

  // 获取所有收藏夹
  const fetchPlaylists = async () => {
    try {
      const data = await wsClient.sendRequest('favorite/list_playlists')
      playlists.value = data
      // 同步缓存以便于快速渲染
      localStorage.setItem('2fmusic_cached_playlists', JSON.stringify(playlists.value))
    } catch (e) {
      console.error('Failed to fetch playlists via WS:', e)
    }
  }

  // 获取当前收藏夹的所有歌曲ID
  const fetchPlaylistSongs = async (playlistId: string) => {
    try {
      const data = await wsClient.sendRequest('favorite/playlist_songs', { playlist_id: playlistId })
      favoriteSongIds.value = data
      if (playlistId === 'default') {
        // 同步缓存
        localStorage.setItem('2fmusic_favs', JSON.stringify(favoriteSongIds.value))
      }
    } catch (e) {
      console.error('Failed to fetch playlist songs via WS:', e)
    }
  }

  // 创建收藏夹
  const createPlaylist = async (name: string) => {
    try {
      await wsClient.sendRequest('favorite/create_playlist', { name })
      await fetchPlaylists()
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '创建异常' }
    }
  }

  // 删除收藏夹
  const deletePlaylist = async (playlistId: string) => {
    try {
      await wsClient.sendRequest('favorite/delete_playlist', { playlist_id: playlistId })
      await fetchPlaylists()
      if (currentPlaylistId.value === playlistId) {
        currentPlaylistId.value = 'default'
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '删除异常' }
    }
  }

  // 添加歌曲到收藏夹
  const addFavorite = async (songId: string, playlistId: string = 'default', title: string = '', artist: string = '') => {
    try {
      await wsClient.sendRequest('favorite/add', { song_id: songId, playlist_id: playlistId, title, artist })
      if (playlistId === currentPlaylistId.value) {
        if (!favoriteSongIds.value.includes(songId)) {
          favoriteSongIds.value.push(songId)
        }
      }
      await fetchPlaylists() // 更新歌曲统计
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '收藏异常' }
    }
  }

  // 从收藏夹移除歌曲
  const removeFavorite = async (songId: string, playlistId: string = 'default') => {
    try {
      await wsClient.sendRequest('favorite/delete', { song_id: songId, playlist_id: playlistId })
      if (playlistId === currentPlaylistId.value) {
        favoriteSongIds.value = favoriteSongIds.value.filter(id => id !== songId)
      }
      await fetchPlaylists() // 更新歌曲统计
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '取消收藏异常' }
    }
  }

  // 批量添加
  const batchAddFavorites = async (songIds: string[], playlistIds: string[], songsInfo: Record<string, { title: string; artist: string }>) => {
    try {
      await wsClient.sendRequest('favorite/batch_add', {
        song_ids: songIds,
        playlist_ids: playlistIds,
        songs: songsInfo
      })
      await fetchPlaylists()
      if (playlistIds.includes(currentPlaylistId.value)) {
        await fetchPlaylistSongs(currentPlaylistId.value)
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '批量收藏异常' }
    }
  }

  // 批量移除
  const batchRemoveFavorites = async (songIds: string[], playlistIds: string[]) => {
    try {
      await wsClient.sendRequest('favorite/batch_delete', {
        song_ids: songIds,
        playlist_ids: playlistIds
      })
      await fetchPlaylists()
      if (playlistIds.includes(currentPlaylistId.value)) {
        await fetchPlaylistSongs(currentPlaylistId.value)
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '批量取消收藏异常' }
    }
  }

  // 批量移动
  const batchMoveFavorites = async (songIds: string[], fromPlaylistId: string, toPlaylistId: string) => {
    try {
      await wsClient.sendRequest('favorite/batch_move', {
        song_ids: songIds,
        from_playlist_id: fromPlaylistId,
        to_playlist_id: toPlaylistId
      })
      await fetchPlaylists()
      if (fromPlaylistId === currentPlaylistId.value || toPlaylistId === currentPlaylistId.value) {
        await fetchPlaylistSongs(currentPlaylistId.value)
      }
      return { success: true }
    } catch (e: any) {
      return { success: false, error: e.message || '批量移动异常' }
    }
  }

  return {
    playlists,
    favoriteSongIds,
    currentPlaylistId,
    fetchPlaylists,
    fetchPlaylistSongs,
    createPlaylist,
    deletePlaylist,
    addFavorite,
    removeFavorite,
    batchAddFavorites,
    batchRemoveFavorites,
    batchMoveFavorites
  }
})
