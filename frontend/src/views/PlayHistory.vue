<template>
  <div class="flex flex-col flex-1 min-h-0 history-view">
    <div
      class="view-header flex justify-between items-center mb-6 max-md:mb-4 [&_.n-button]:max-md:w-[38px] [&_.n-button]:max-md:h-[38px] [&_.n-button]:max-md:p-0 [&_.n-button]:max-md:justify-center [&_.n-button]:max-md:rounded-full [&_.n-button_.n-button__content]:max-md:hidden! [&_.n-button_.n-button__icon]:max-md:m-0!">
      <h1 class="view-title">播放记录</h1>
      <n-button v-if="historyList.length > 0" round type="error" text @click="showClearConfirm = true">
        <template #icon>
          <SvgIcon name="trash" />
        </template>
        清除全部记录
      </n-button>
    </div>

    <div class="song-list-container">
      <div v-if="historyList.length === 0" class="empty-state">
        <SvgIcon name="history" />
        <h3>暂无播放记录</h3>
        <p>你在播放器里听过歌后，记录会自动在此呈现</p>
      </div>

      <template v-else>
        <!-- 固定在顶部的极简表头 -->
        <div class="song-grid-header max-md:hidden!" :style="{ paddingRight: `${16 + scrollbarWidth}px` }">
          <div class="col-title">标题</div>
          <div class="col-artist">歌手</div>
          <div class="col-album">专辑</div>
          <div class="col-time">播放时间</div>
          <div class="col-spacer"></div>
          <div class="col-actions"></div>
        </div>

        <!-- 高性能虚拟列表滚动区 -->
        <n-virtual-list ref="virtualListRef" class="song-list-scroll-area flex-1 min-h-0 overflow-hidden" :item-size="64"
          :items="historyList" key-field="key" :item-resizable="false" :ignore-item-resize="true" @resize="updateScrollbarWidth">
          <template #default="{ item }">
            <div
              class="song-row max-md:grid! max-md:grid-cols-[auto_1fr_auto_auto] max-md:grid-rows-[auto_auto] max-md:[grid-template-areas:'cover_title_time_action'_'cover_artist_time_action'] max-md:items-center max-md:p-[8px_12px] max-md:gap-x-3 max-md:gap-y-[2px]"
              :class="{ playing: playerStore.currentSong?.id === item.song.id }" @click="handleRowClick(item.song)"
              @dblclick="playSong(item.song)">
              <div class="col-title max-md:contents!">
                <div class="cover-box max-md:[grid-area:cover] max-md:row-[span_2] max-md:w-10 max-md:h-10"
                  @click.stop="handlePlayBtnClick(item.song)">
                  <img class="w-full h-full object-cover" v-cached-src="{ id: item.song.id, src: item.song.album_art }"
                    loading="lazy" alt="Cover" />
                  <div class="play-hover">
                    <SvgIcon
                      :name="playerStore.currentSong?.id === item.song.id && playerStore.isPlaying ? 'pause' : 'play'" />
                  </div>
                </div>
                <div class="title-text-box max-md:[grid-area:title] max-md:self-end max-md:overflow-hidden">
                  <span class="song-name">{{ item.song.title }}</span>
                </div>
              </div>
              <div
                class="col-artist max-md:[grid-area:artist] max-md:self-start max-md:min-w-0 max-md:text-[12px] max-md:text-body-muted max-md:m-0 max-md:p-0"
                :title="item.song.artist">{{ item.song.artist }}</div>
              <div class="col-album max-md:hidden!" :title="item.song.album">{{ item.song.album || '-' }}</div>
              <div
                class="col-time max-md:[grid-area:time] max-md:row-[span_2] max-md:w-auto max-md:flex max-md:items-center max-md:text-[11px] max-md:text-body-muted">
                {{ formatPlayTime(item.time) }}</div>

              <!-- 弹性占位空列 -->
              <div class="col-spacer max-md:hidden!"></div>

          <div
                class="col-actions max-md:[grid-area:action] max-md:row-[span_2] max-md:w-auto max-md:flex max-md:items-center"
                @click.stop>
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
import { ref, onMounted, computed, watch, nextTick } from 'vue'
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

// 历史数据绑定唯一 key
const historyList = computed(() => {
  return historyStore.historyList.map(item => ({
    ...item,
    key: `${item.song.id}-${item.time}`
  }))
})

const loadHistory = () => {
  if (localStorage.getItem('2fmusic_password')) {
    historyStore.fetchHistory()
  }
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
    if (playerStore.playlist.length === 0) {
      playerStore.playlist = historyList.value.map(h => h.song)
    }
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

const scrollbarWidth = ref(0)
const virtualListRef = ref<any>(null)

const updateScrollbarWidth = () => {
  if (virtualListRef.value?.$el) {
    const el = virtualListRef.value.$el
    scrollbarWidth.value = el.offsetWidth - el.clientWidth
  }
}

watch(() => historyList.value, () => {
  nextTick(() => {
    setTimeout(updateScrollbarWidth, 50)
  })
}, { deep: true })

onMounted(() => {
  loadHistory()
  setTimeout(updateScrollbarWidth, 200)
})
</script>