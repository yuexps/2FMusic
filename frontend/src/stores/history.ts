import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Song } from '../types'
import { wsClient } from '../api/ws'

export interface HistoryItem {
  song: Song
  time: number
}

export const useHistoryStore = defineStore('history', () => {
  const historyList = ref<HistoryItem[]>([])
  const isLoading = ref(false)

  const clearUserData = () => {
    historyList.value = []
    localStorage.removeItem('2fmusic_history')
  }

  // 获取云端历史记录
  const fetchHistory = async () => {
    isLoading.value = true
    try {
      const data = await wsClient.sendRequest('history/get')
      historyList.value = Array.isArray(data) ? data : []
      localStorage.setItem('2fmusic_history', JSON.stringify(historyList.value))
    } catch (e: any) {
      if (!e?.isWSClosed) {
        console.error('通过 WebSocket 获取播放历史失败:', e)
        const saved = localStorage.getItem('2fmusic_history')
        if (saved) {
          historyList.value = JSON.parse(saved)
        }
      }
    } finally {
      isLoading.value = false
    }
  }

  // 添加播放记录
  const addHistory = async (songId: string) => {
    try {
      await wsClient.sendRequest('history/add', { song_id: songId })
      await fetchHistory() // 成功添加后静默更新最新列表
    } catch (e) {
      console.error('通过 WebSocket 添加播放历史失败:', e)
    }
  }

  // 清空播放历史
  const clearHistory = async () => {
    try {
      await wsClient.sendRequest('history/clear')
      historyList.value = []
      localStorage.removeItem('2fmusic_history')
      return { success: true }
    } catch (e: any) {
      console.error('通过 WebSocket 清除所有播放历史失败:', e)
      return { success: false, error: e.message || '清空失败' }
    }
  }

  // 删除单条播放记录
  const removeHistory = async (songId: string, playTime: number) => {
    try {
      await wsClient.sendRequest('history/remove', { song_id: songId, play_time: playTime })
      historyList.value = historyList.value.filter(item => !(item.song.id === songId && item.time === playTime))
      localStorage.setItem('2fmusic_history', JSON.stringify(historyList.value))
      return { success: true }
    } catch (e: any) {
      console.error('通过 WebSocket 移除单条播放历史失败:', e)
      return { success: false, error: e.message || '移除记录失败' }
    }
  }

  return {
    historyList,
    isLoading,
    clearUserData,
    fetchHistory,
    addHistory,
    clearHistory,
    removeHistory
  }
})
