<template>
  <div
    class="player-bar-container glass-panel flex items-center justify-between px-6 box-border shrink-0 h-20 relative">
    <!-- 移动端专用顶部极细进度指示条 (100% 空间利用率，贴顶展示播放进度) -->
    <div
      class="absolute top-0 left-0 w-full h-[2px] bg-black/5 dark:bg-white/10 hidden max-md:block overflow-hidden pointer-events-none">
      <div class="h-full bg-primary dark:bg-primary-on-dark transition-all duration-100 ease-linear"
        :style="{ width: (playerStore.currentTime / (playerStore.duration || 1)) * 100 + '%' }"></div>
    </div>

    <!-- 左侧：歌曲信息 -->
    <div
      class="flex items-center gap-3 w-1/4 min-w-45 cursor-pointer overflow-hidden max-md:min-w-0 max-md:w-auto max-md:flex-1 max-md:pr-4"
      @click="emit('open-lyrics')" title="点击查看歌词">
      <img v-cached-src="{ id: playerStore.currentSong?.id, src: playerStore.currentSong?.album_art }"
        class="w-12 h-12 object-cover shrink-0 rounded-md" alt="Cover" />
      <div class="overflow-hidden">
        <div class="text-sm font-semibold text-ink truncate mb-0.5" :title="playerStore.currentSong?.title || '等待播放'">
          {{ playerStore.currentSong?.title || '等待播放' }}
        </div>
        <div class="text-xs text-body-muted dark:text-ink truncate"
          :title="playerStore.currentSong?.artist || '2FMusic'">
          {{ playerStore.currentSong?.artist || '2FMusic' }}
        </div>
      </div>
    </div>

    <!-- 中间：播放控制及进度 (移动端下直接隐藏以释出空间给歌名标题) -->
    <div class="flex flex-col items-center gap-2 w-[45%] max-w-145 max-md:hidden">
      <div class="flex items-center gap-8">
        <!-- 播放模式切换 -->
        <n-button circle text class="text-lg! max-md:hidden!" @click="togglePlayMode" :title="modeTitle">
          <template #icon>
            <SvgIcon :name="modeIcon" />
          </template>
        </n-button>

        <n-button circle text class="text-lg!" @click="playerStore.prev" title="上一首">
          <template #icon>
            <SvgIcon name="backward" />
          </template>
        </n-button>

        <n-button circle type="primary" class="w-10 h-10" @click="playerStore.togglePlay"
          :title="playerStore.isPlaying ? '暂停' : '播放'">
          <template #icon>
            <SvgIcon :name="playerStore.isPlaying ? 'pause' : 'play'" />
          </template>
        </n-button>

        <n-button circle text class="text-lg!" @click="playerStore.next" title="下一首">
          <template #icon>
            <SvgIcon name="forward" />
          </template>
        </n-button>
      </div>

      <div class="flex items-center gap-3 w-full">
        <span class="text-[11px] text-body-muted min-w-8 text-center">{{ formatTime(playerStore.currentTime) }}</span>
        <n-slider
          class="[&_.n-slider-handle]:w-3 [&_.n-slider-handle]:h-3 [&_.n-slider-handle]:bg-primary dark:[&_.n-slider-handle]:bg-primary-on-dark [&_.n-slider-handle]:border-2 [&_.n-slider-handle]:border-white [&_.n-slider-rail]:h-1!"
          v-model:value="sliderTime" :max="playerStore.duration || 100" :step="0.1" :tooltip="false"
          @update:value="onSliderChange" />
        <span class="text-[11px] text-body-muted min-w-8 text-center">{{ formatTime(playerStore.duration) }}</span>
      </div>
    </div>

    <!-- 右侧：控制与音量 (移动端下合并展示核心操控) -->
    <div
      class="flex items-center gap-3 w-1/4 justify-end min-w-37.5 max-md:min-w-0 max-md:w-auto max-md:justify-end max-md:flex-none max-md:gap-3.5">
      <!-- 移动端专用极简控制组件 (上一首、播放/暂停 和 下一首) -->
      <div class="hidden max-md:flex items-center gap-3.5">
        <n-button circle text class="text-xl!" @click="playerStore.prev" title="上一首">
          <template #icon>
            <SvgIcon name="backward" />
          </template>
        </n-button>
        <!-- 主播放核心按键 -->
        <n-button circle type="primary" class="w-9 h-9 flex items-center justify-center" @click="playerStore.togglePlay"
          :title="playerStore.isPlaying ? '暂停' : '播放'">
          <template #icon>
            <SvgIcon :name="playerStore.isPlaying ? 'pause' : 'play'" class="text-white!" />
          </template>
        </n-button>
        <n-button circle text class="text-xl!" @click="playerStore.next" title="下一首">
          <template #icon>
            <SvgIcon name="forward" />
          </template>
        </n-button>
      </div>

      <!-- 桌面端音量调节 -->
      <div class="flex items-center gap-2 max-md:hidden">
        <n-button circle text class="text-lg!" @click="toggleMute" title="静音">
          <template #icon>
            <SvgIcon :name="volumeIcon" />
          </template>
        </n-button>
        <div class="w-22.5">
          <n-slider
            class="[&_.n-slider-handle]:w-3 [&_.n-slider-handle]:h-3 [&_.n-slider-handle]:bg-primary dark:[&_.n-slider-handle]:bg-primary-on-dark [&_.n-slider-handle]:border-2 [&_.n-slider-handle]:border-white [&_.n-slider-rail]:h-1!"
            v-model:value="volumePercent" :max="100" :step="1" :tooltip="false" @update:value="onVolumeChange" />
        </div>
      </div>

      <!-- 播放队列纯图标按钮 (移动端与桌面端通用) -->
      <n-button circle text class="text-lg!" :class="{ 'text-primary dark:text-primary-on-dark': showQueue }"
        @click="showQueue = !showQueue" title="播放队列">
        <template #icon>
          <SvgIcon name="tasks" />
        </template>
      </n-button>
    </div>

    <!-- 播放队列抽屉 (Popover/Drawer) -->
    <div v-if="showQueue"
      class="absolute bottom-22.5 right-6 w-[320px] h-100 max-h-[calc(100vh-120px)] max-h-700:h-[280px]! glass-panel p-4 box-border flex flex-col rounded-2xl z-50 max-md:right-4 max-md:left-4 max-md:w-auto">
      <n-tabs type="segment" size="small" v-model:value="queueTab" class="mb-3">
        <n-tab name="playlist" :tab="`当前列表 (${playerStore.playlist.length})`" />
        <n-tab name="queue" :tab="`待播队列 (${playerStore.queue.length})`" />
      </n-tabs>

      <div class="flex justify-between items-center mb-3 border-b border-hairline pb-2">
        <h3 class="m-0 text-sm font-semibold text-ink">{{ queueTab === 'playlist' ? '当前列表' : '待播队列' }}</h3>
        <n-button v-if="queueTab === 'queue'" round size="tiny" secondary type="error"
          @click="playerStore.clearQueue">清空</n-button>
      </div>

      <div class="flex-1 flex flex-col min-height-0 overflow-hidden">
        <!-- 标签页：当前列表 -->
        <template v-if="queueTab === 'playlist'">
          <div v-if="playerStore.playlist.length === 0" class="text-xs text-body-muted text-center py-8">
            播放列表为空
          </div>
          <n-virtual-list v-else class="-mr-4 pr-4" style="flex:1;min-height:0" :item-size="48"
            :items="playerStore.playlist" key-field="id">
            <template #default="{ item: song }">
              <div
                class="flex items-center px-2 h-12 box-border rounded-lg cursor-pointer transition-colors duration-150"
                :class="[
                  playerStore.currentSong?.id === song.id
                    ? 'bg-primary/5 dark:bg-primary-on-dark/5 text-primary dark:text-primary-on-dark font-semibold'
                    : 'text-ink hover:bg-black/3 dark:hover:bg-white/5'
                ]" @click="handlePlayFromPlaylist(song)">
                <div class="flex-1 text-xs truncate">{{ song.title }}</div>
                <div class="text-[10px] text-body-muted mr-2 truncate max-w-20"
                  :class="{ 'text-primary dark:text-primary-on-dark opacity-80': playerStore.currentSong?.id === song.id }">
                  {{ song.artist }}</div>
                <div v-if="playerStore.currentSong?.id === song.id"
                  class="text-primary dark:text-primary-on-dark text-xs ml-1 flex items-center">
                  <SvgIcon name="volume-up" />
                </div>
              </div>
            </template>
          </n-virtual-list>
        </template>

        <!-- 标签页：待播队列 -->
        <template v-else>
          <div v-if="playerStore.queue.length === 0" class="text-xs text-body-muted text-center py-8">
            队列为空，从歌曲列表中右键"添加至队列"
          </div>
          <n-virtual-list v-else class="-mr-4 pr-4" style="flex:1;min-height:0" :item-size="48"
            :items="playerStore.queue" key-field="id">
            <template #default="{ item: song }">
              <div
                class="flex items-center px-2 h-12 box-border rounded-lg cursor-pointer text-ink hover:bg-black/3 dark:hover:bg-white/5 transition-colors duration-150"
                @click="playFromQueue(song)">
                <div class="flex-1 text-xs truncate">{{ song.title }}</div>
                <div class="text-[10px] text-body-muted mr-2 truncate max-w-20">{{ song.artist }}</div>
                <n-button circle text size="small" class="text-body-muted!"
                  @click.stop="playerStore.removeFromQueue(song.id)">
                  <template #icon>
                    <SvgIcon name="times" />
                  </template>
                </n-button>
              </div>
            </template>
          </n-virtual-list>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { usePlayerStore } from '../stores/player'
import { NSlider, NButton, NTabs, NTab, NVirtualList } from 'naive-ui'
import type { Song } from '../types'
const playerStore = usePlayerStore()
const showQueue = ref(false)
const queueTab = ref<'playlist' | 'queue'>('playlist')

const emit = defineEmits(['open-lyrics'])

// 时间进度条双向绑定与节流
const sliderTime = ref(0)
let isUserSeeking = false

watch(() => playerStore.currentTime, (val) => {
  if (!isUserSeeking) {
    sliderTime.value = val
  }
})

const onSliderChange = (val: number) => {
  playerStore.seek(val)
}

// 音量双向绑定
const volumePercent = ref(100)
watch(() => playerStore.volume, (val) => {
  volumePercent.value = Math.round(val * 100)
}, { immediate: true })

const onVolumeChange = (val: number) => {
  playerStore.setVolume(val / 100)
}

const prevVolume = ref(1.0)
const toggleMute = () => {
  if (playerStore.volume > 0) {
    prevVolume.value = playerStore.volume
    playerStore.setVolume(0)
  } else {
    playerStore.setVolume(prevVolume.value)
  }
}

const volumeIcon = computed(() => {
  const vol = playerStore.volume
  if (vol === 0) return 'volume-mute'
  return 'volume-up'
})

// 播放模式切换
const togglePlayMode = () => {
  const modes: ('list' | 'single' | 'random')[] = ['list', 'single', 'random']
  const nextIdx = (modes.indexOf(playerStore.playMode) + 1) % modes.length
  playerStore.playMode = modes[nextIdx]
  // 同步到本地存储
  const saved = localStorage.getItem('2fmusic_state')
  const state = saved ? JSON.parse(saved) : {}
  state.playMode = playerStore.playMode
  localStorage.setItem('2fmusic_state', JSON.stringify(state))
}

const modeIcon = computed(() => {
  if (playerStore.playMode === 'single') return 'redo-alt'
  if (playerStore.playMode === 'random') return 'random'
  return 'redo'
})

const modeTitle = computed(() => {
  if (playerStore.playMode === 'single') return '单曲循环'
  if (playerStore.playMode === 'random') return '随机播放'
  return '列表循环'
})

const playFromQueue = (song: Song) => {
  playerStore.playSong(song)
  playerStore.removeFromQueue(song.id)
  showQueue.value = false
}

const handlePlayFromPlaylist = (song: Song) => {
  if (playerStore.currentSong?.id === song.id) {
    playerStore.togglePlay()
  } else {
    playerStore.playSong(song)
  }
}

// 格式化秒数为 0:00
const formatTime = (secs: number) => {
  if (isNaN(secs)) return '0:00'
  const m = Math.floor(secs / 60)
  const s = Math.floor(secs % 60)
  return `${m}:${s < 10 ? '0' : ''}${s}`
}

onMounted(() => {
  playerStore.init()
})
</script>
