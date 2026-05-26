<template>
  <div class="flex flex-col flex-1 min-h-0 history-view">
    <div class="view-header">
      <h1 class="view-title">播放记录</h1>
      <n-button v-if="historyList.length > 0" round type="error" text @click="showClearConfirm = true">
        <template #icon>
          <SvgIcon name="trash" />
        </template>
        清除全部记录
      </n-button>
    </div>

    <!-- 历史记录列表 -->
    <div class="song-list-container">
      <div v-if="historyList.length === 0" class="empty-state">
        <SvgIcon name="history" />
        <h3>暂无播放记录</h3>
        <p>你在播放器里听过歌后，记录会自动在此呈现</p>
      </div>

      <template v-else>
        <!-- 固定在顶部的极简表头 -->
        <div class="song-grid-header">
          <div class="col-title">标题</div>
          <div class="col-artist">歌手</div>
          <div class="col-album">专辑</div>
          <div class="col-time">播放时间</div>
          <div class="col-actions"></div>
        </div>

        <!-- 高性能虚拟列表滚动区 -->
        <n-virtual-list class="song-list-scroll-area flex-1 min-h-0 overflow-hidden" :item-size="64"
          :items="historyList" key-field="key" :item-resizable="false">
          <template #default="{ item }">
            <div class="song-row" :class="{ playing: playerStore.currentSong?.id === item.song.id }"
              @click="handleRowClick(item.song)" @dblclick="playSong(item.song)">
              <div class="col-title">
                <div class="cover-box" @click.stop="handlePlayBtnClick(item.song)">
                  <img v-cached-src="{ id: item.song.id, src: item.song.album_art }" loading="lazy" alt="Cover" />
                  <div class="play-hover">
                    <SvgIcon
                      :name="playerStore.currentSong?.id === item.song.id && playerStore.isPlaying ? 'pause' : 'play'" />
                  </div>
                </div>
                <div class="title-text-box">
                  <span class="song-name">{{ item.song.title }}</span>
                </div>
              </div>
              <div class="col-artist" :title="item.song.artist">{{ item.song.artist }}</div>
              <div class="col-album" :title="item.song.album">{{ item.song.album || '-' }}</div>
              <div class="col-time">{{ formatPlayTime(item.time) }}</div>

              <!-- 右侧操作 -->
              <div class="col-actions" @click.stop>
                <n-dropdown trigger="click" :options="getRowDropdownOptions(item.song, item.time)"
                  @select="(key) => handleRowAction(key, item.song, item.time)">
                  <n-button circle text :depth="3" class="row-menu-btn">
                    <template #icon>
                      <SvgIcon name="ellipsis-h" />
                    </template>
                  </n-button>
                </n-dropdown>
              </div>
            </div>
          </template>
        </n-virtual-list>
      </template>
    </div>

    <!-- 移除历史记录确认弹窗 -->
    <n-modal v-model:show="showRemoveConfirm" preset="dialog" title="确认移除播放记录">
      <div>
        确定要从播放历史记录中移除这首歌曲吗？这不会删除您的物理音乐文件。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showRemoveConfirm = false">取消</n-button>
          <n-button round type="error" @click="confirmRemoveHistory">确认移除</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 清空播放记录确认弹窗 -->
    <n-modal v-model:show="showClearConfirm" preset="dialog" title="确认清空播放记录">
      <div>
        确定要清空全部的播放历史记录吗？这不会删除您的物理音乐文件。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showClearConfirm = false">取消</n-button>
          <n-button round type="error" @click="confirmClearHistory">确认清空</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { usePlayerStore } from '../stores/player'
import { useHistoryStore } from '../stores/history'
import { NDropdown, NModal, NButton, NSpace, NVirtualList, useMessage } from 'naive-ui'
import type { Song } from '../types'

const showRemoveConfirm = ref(false)
const itemToRemove = ref<{ song: Song; time: number } | null>(null)
const showClearConfirm = ref(false)

const playerStore = usePlayerStore()
const historyStore = useHistoryStore()
const message = useMessage()

// 将页面数据绑定至包含唯一 key 属性的计算属性
const historyList = computed(() => {
  return historyStore.historyList.map(item => ({
    ...item,
    key: `${item.song.id}-${item.time}`
  }))
})

const loadHistory = () => {
  historyStore.fetchHistory()
}

const playSong = (song: Song) => {
  // 从历史播放列表中抽取歌曲清单作为播放上下文
  const songs = historyList.value.map(h => h.song)
  playerStore.playSong(song, songs)
}

const handleRowClick = (song: Song) => {
  const isMobile = window.innerWidth <= 768
  if (isMobile) {
    playSong(song)
  }
}

const handlePlayBtnClick = (song: Song) => {
  if (playerStore.currentSong?.id === song.id) {
    playerStore.togglePlay()
  } else {
    playSong(song)
  }
}

const confirmClearHistory = async () => {
  const res = await historyStore.clearHistory()
  showClearConfirm.value = false
  if (res.success) {
    message.success('播放历史记录已成功全部清空')
  } else {
    message.error(res.error || '清空失败')
  }
}

const getRowDropdownOptions = (song: Song, time: number) => {
  if (song.id || time) { }
  return [
    { label: '播放', key: 'play' },
    { label: '添加至队列', key: 'queue' },
    { label: '从历史记录中移除', key: 'remove_history' }
  ]
}

const handleRowAction = (key: string, song: Song, time: number) => {
  if (key === 'play') {
    playSong(song)
  } else if (key === 'queue') {
    playerStore.addToQueue(song)
    message.success('已添加到播放队列')
  } else if (key === 'remove_history') {
    itemToRemove.value = { song, time }
    showRemoveConfirm.value = true
  }
}

const confirmRemoveHistory = async () => {
  if (!itemToRemove.value) return
  const { song, time } = itemToRemove.value
  const res = await historyStore.removeHistory(song.id, time)
  showRemoveConfirm.value = false
  if (res.success) {
    message.success('已成功从播放历史中移除')
  } else {
    message.error(res.error || '移除失败')
  }
  itemToRemove.value = null
}

// 格式化历史播放时间
const formatPlayTime = (timestamp: number) => {
  const date = new Date(timestamp)
  const now = new Date()

  const isToday = date.toDateString() === now.toDateString()

  const pad = (n: number) => n < 10 ? '0' + n : n
  const hours = pad(date.getHours())
  const minutes = pad(date.getMinutes())

  if (isToday) {
    return `今天 ${hours}:${minutes}`
  } else {
    const month = date.getMonth() + 1
    const day = date.getDate()
    return `${month}月${day}日 ${hours}:${minutes}`
  }
}

onMounted(() => {
  loadHistory()
})
</script>

<style scoped>
/* 仅留存移动端响应式网格重构和虚拟列表的特定适配 */

@media (max-width: 768px) {
  .view-header {
    margin-bottom: 16px;
  }

  .view-header :deep(.action-btn) {
    width: 38px;
    height: 38px;
    padding: 0;
    justify-content: center;
    border-radius: 50%;
  }

  .view-header :deep(.action-btn .n-button__content) {
    display: none !important;
  }

  .view-header :deep(.action-btn .n-button__icon) {
    margin: 0 !important;
  }

  .song-grid-header {
    display: none;
  }

  .song-row {
    display: grid !important;
    grid-template-areas:
      "cover title time action"
      "cover artist time action";
    grid-template-columns: auto 1fr auto auto;
    grid-template-rows: auto auto;
    align-items: center;
    padding: 8px 12px;
    gap: 2px 12px;
  }

  .col-title {
    display: contents !important;
  }

  .cover-box {
    grid-area: cover;
    grid-row: span 2;
    width: 40px;
    height: 40px;
  }

  .title-text-box {
    grid-area: title;
    align-self: end;
    overflow: hidden;
  }

  .col-artist {
    grid-area: artist;
    align-self: start;
    min-width: 0;
    font-size: 12px;
    color: var(--body-muted);
    margin: 0;
    padding: 0;
  }

  .col-album {
    display: none !important;
  }

  .col-time {
    grid-area: time;
    grid-row: span 2;
    display: flex;
    align-items: center;
    width: auto;
    font-size: 11px;
    color: var(--body-muted);
  }

  .col-actions {
    grid-area: action;
    grid-row: span 2;
    display: flex;
    align-items: center;
    width: auto;
  }
}

/* 锁定外层滚动 - 虚拟列表自行处理滚动 */
:global(.main-scroll:has(.history-view)) {
  overflow: hidden !important;
  display: flex !important;
  flex-direction: column !important;
  padding-bottom: 1px !important;
}
</style>
