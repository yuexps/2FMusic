<template>
  <div class="fixed left-0 w-[calc(100vw/var(--ui-scale,1))] h-[calc(100vh/var(--ui-scale,1))] bg-black z-1000 flex flex-col transition-[top,visibility] duration-400 ease-[cubic-bezier(0.25,1,0.5,1)] overflow-hidden text-white" :class="[ show ? 'top-0 visible pointer-events-auto' : 'top-[120vh] invisible pointer-events-none' ]">
    <!-- 虚化背景封面 -->
    <div class="absolute inset-0 -z-1 overflow-hidden">
      <img v-cached-src="{ id: playerStore.currentSong?.id, src: playerStore.currentSong?.album_art }"
        class="absolute top-[-10%] left-[-10%] w-[120%] h-[120%] object-cover blur-[50px] brightness-[0.4] saturate-[1.4] scale-110" alt="Background" />
      <div class="absolute inset-0 bg-linear-to-b from-black/40 to-black/70"></div>
    </div>

    <!-- 头部：关闭及状态 -->
    <header class="h-16 flex items-center justify-between px-6 box-border lg:px-20 relative z-60">
      <button class="bg-transparent border-none text-white text-[20px] cursor-pointer opacity-60 transition-all duration-150 hover:opacity-100 hover:translate-y-0.5 flex items-center justify-center" @click="emit('close')" title="收起">
        <SvgIcon name="chevron-down" />
      </button>

      <!-- 右侧三点菜单 -->
      <n-dropdown trigger="click" :options="dropdownOptions" @select="handleMenuSelect">
        <button class="bg-transparent border-none text-white text-[20px] cursor-pointer opacity-60 w-9 h-9 rounded-full flex items-center justify-center transition-all duration-150 hover:opacity-100 hover:bg-white/12 active:scale-[0.93]" title="更多操作">
          <SvgIcon name="ellipsis-h" />
        </button>
      </n-dropdown>
    </header>

    <!-- 主体：双列布局 -->
    <main class="fp-body flex-1 flex px-20 py-10 box-border overflow-hidden gap-12 xl:gap-16 max-md:flex-col max-md:p-5 max-md:gap-4 md:max-lg:px-10 md:max-lg:py-7 md:max-lg:gap-10">
      <!-- 左侧：封面和元数据 -->
      <div class="flex-1 flex flex-col justify-center items-center text-center min-w-0 max-md:flex-[0_0_auto] max-md:flex-row max-md:items-center max-md:text-left max-md:gap-4 max-md:w-full">
        <div class="w-[min(360px,45vw,42vh)] lg:w-[min(380px,45vw,42vh)] xl:w-[min(440px,48vw,46vh)] 2xl:w-[min(480px,50vw,50vh)] aspect-square mb-8 flex justify-center items-center max-md:w-18 max-md:h-18 max-md:mb-0 max-md:shrink-0 md:max-lg:w-[min(240px,40vw,35vh)] md:max-lg:mb-4 xl:mb-10 shadow-[0_20px_40px_rgba(0,0,0,0.35)] dark:shadow-[0_20px_40px_rgba(0,0,0,0.55)] max-md:shadow-[0_10px_20px_rgba(0,0,0,0.25)]">
          <img v-cached-src="{ id: playerStore.currentSong?.id, src: playerStore.currentSong?.album_art }"
            class="w-full h-full object-contain bg-black/5 dark:bg-white/5 rounded-md" alt="Cover" />
        </div>
        <div class="w-full max-w-[min(480px,90%)] text-white max-md:flex-1 max-md:min-w-0 max-md:text-left">
          <h2 class="font-display text-[22px] lg:text-[22px] xl:text-[26px] 2xl:text-[30px] font-semibold m-0 mb-2 xl:mb-3 tracking-[-0.3px] line-clamp-2 wrap-break-word leading-snug py-0.5 max-md:text-base max-md:mb-1 md:max-lg:text-lg" :title="playerStore.currentSong?.title">{{ playerStore.currentSong?.title || '未知标题' }}</h2>
          <p class="text-white/60 text-sm lg:text-sm xl:text-base 2xl:text-lg m-0 line-clamp-1 wrap-break-word max-md:text-xs md:max-lg:text-sm" :title="playerStore.currentSong?.artist">{{ playerStore.currentSong?.artist || '未知艺术家' }}</p>
        </div>
      </div>

      <!-- Right column: lyrics scroll -->
      <div class="fp-right flex-[1.4] flex flex-col justify-center overflow-hidden max-md:flex-1 max-md:justify-start max-md:h-0">
        <div ref="lyricsContainer" class="lyrics-scroll-container h-[85%] overflow-y-auto py-10 box-border mask-[linear-gradient(180deg,transparent_0%,#000_15%,#000_85%,transparent_100%)] max-md:h-full max-md:py-5" :class="{ 'no-lyrics': lyricLines.length === 0 }">
          <div v-if="lyricLines.length === 0" class="empty-lyrics flex items-center justify-center h-full">
            <p class="text-[20px] font-semibold text-white px-4 py-3 text-center active">暂无歌词</p>
          </div>
          <div v-else v-for="(line, idx) in lyricLines" :key="idx"
            :ref="el => { if (el) lyricElements[idx] = el as HTMLElement }" class="lyric-line group/lyric text-[20px] lg:text-[19px] xl:text-[21px] 2xl:text-[24px] font-semibold text-white/45 px-4 py-3 rounded-lg cursor-pointer transition-all duration-300 origin-center leading-relaxed text-center hover:text-white/80 hover:bg-white/5 max-md:text-base max-md:px-2 max-md:py-2.5 max-md:leading-snug max-md:hover:bg-transparent max-md:hover:text-white/45 select-none touch-none"
            :class="{ 'text-white! text-[24px] lg:text-[23px] xl:text-[25px] 2xl:text-[28px] scale-[1.03] cursor-default hover:bg-transparent max-md:text-[18px]': currentLyricIndex === idx }" @click="seekToLyric(line.time)">
            <template v-if="getLineTexts(line).length > 1">
              <span class="block lyric-main">
                <template v-if="line.isYrc && currentLyricIndex === idx && line.words && line.words.length > 0">
                  <span v-for="(word, wIdx) in line.words" :key="wIdx" class="yrc-word" :class="getWordClass(word)" :style="getWordStyle(word)">{{ word.text }}</span>
                </template>
                <template v-else>
                  {{ getLineTexts(line)[0] }}
                </template>
              </span>
              <span class="block text-[0.75em] font-normal text-white/48 mt-1.5 tracking-wide transition-colors duration-300 group-hover/lyric:text-white/65 max-md:mt-1 max-md:text-[0.78em]" :class="{ 'text-white/72!': currentLyricIndex === idx }">{{ getLineTexts(line)[1] }}</span>
            </template>
            <template v-else>
              <span class="block lyric-main">
                <template v-if="line.isYrc && currentLyricIndex === idx && line.words && line.words.length > 0">
                  <span v-for="(word, wIdx) in line.words" :key="wIdx" class="yrc-word" :class="getWordClass(word)" :style="getWordStyle(word)">{{ word.text }}</span>
                </template>
                <template v-else>
                  {{ getLineTexts(line)[0] }}
                </template>
              </span>
            </template>
          </div>
        </div>
      </div>
    </main>

    <!-- 底部：控制台 -->
    <footer class="h-40 flex flex-col items-center px-20 pb-10 box-border gap-4 max-md:px-5 max-md:pb-5 max-md:h-32.5">
      <div class="flex items-center gap-4 w-full max-w-180">
        <span class="text-[11px] opacity-50 min-w-9 text-center">{{ formatTime(playerStore.currentTime) }}</span>
        <n-slider v-model:value="sliderTime" :max="playerStore.duration || 100" :step="0.1" :tooltip="false"
          @update:value="onSliderChange" />
        <span class="text-[11px] opacity-50 min-w-9 text-center">{{ formatTime(playerStore.duration) }}</span>
      </div>

      <div class="flex items-center gap-8 max-md:gap-5">
        <button class="bg-transparent border-none text-white opacity-60 text-lg cursor-pointer p-3 rounded-full flex items-center justify-center transition-all duration-150 hover:opacity-100 hover:bg-white/10 active:scale-95" @click="togglePlayMode" :title="modeTitle">
          <SvgIcon :name="modeIcon" />
        </button>

        <button class="bg-transparent border-none text-white opacity-60 text-lg cursor-pointer p-3 rounded-full flex items-center justify-center transition-all duration-150 hover:opacity-100 hover:bg-white/10 active:scale-95" @click="playerStore.prev" title="上一首">
          <SvgIcon name="backward" />
        </button>

        <button class="w-14 h-14 rounded-full bg-white text-black border-none flex items-center justify-center text-[20px] cursor-pointer transition-all duration-150 hover:scale-105 hover:bg-[#f5f5f7] active:scale-95" @click="playerStore.togglePlay" :title="playerStore.isPlaying ? '暂停' : '播放'">
          <SvgIcon :name="playerStore.isPlaying ? 'pause' : 'play'" />
        </button>

        <button class="bg-transparent border-none text-white opacity-60 text-lg cursor-pointer p-3 rounded-full flex items-center justify-center transition-all duration-150 hover:opacity-100 hover:bg-white/10 active:scale-95" @click="playerStore.next" title="下一首">
          <SvgIcon name="forward" />
        </button>

        <button class="bg-transparent border-none text-white opacity-60 text-lg cursor-pointer p-3 rounded-full flex items-center justify-center transition-all duration-150 hover:opacity-100 hover:bg-white/10 active:scale-95" :class="{ 'text-vip! opacity-100!': isFavorited }" @click="toggleFavorite"
          :title="isFavorited ? '取消收藏' : '添加收藏'">
          <SvgIcon :name="isFavorited ? 'heart' : 'heart-o'" />
        </button>
      </div>
    </footer>

    <!-- 歌曲详情弹窗 -->
    <n-modal v-model:show="showInfoModal" display-directive="show">
      <div class="glass-card w-[min(480px,90vw)] bg-dialog border border-dialog rounded-xl p-6 text-ink shadow-[0_20px_50px_rgba(0,0,0,0.12)] box-border">
        <h3 class="m-0 mb-5 text-lg font-semibold text-center tracking-wider text-ink">歌曲详情</h3>
        <div class="flex flex-col gap-4">
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0">
            <span class="text-[13px] text-body-muted shrink-0 w-15">标题</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]" :title="playerStore.currentSong?.title">{{ playerStore.currentSong?.title || '未知'
              }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0">
            <span class="text-[13px] text-body-muted shrink-0 w-15">歌手</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]" :title="playerStore.currentSong?.artist">{{ playerStore.currentSong?.artist || '未知'
              }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0">
            <span class="text-[13px] text-body-muted shrink-0 w-15">专辑</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]" :title="playerStore.currentSong?.album">{{ playerStore.currentSong?.album || '未知'
              }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0">
            <span class="text-[13px] text-body-muted shrink-0 w-15">文件名</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]" :title="playerStore.currentSong?.filename">{{ playerStore.currentSong?.filename ||
              '未知'
              }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0" v-if="playerStore.currentSong?.size">
            <span class="text-[13px] text-body-muted shrink-0 w-15">大小</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]">{{ formatSize(playerStore.currentSong.size) }}</span>
          </div>
          <div class="flex justify-between items-center border-b border-border-card pb-3 last:border-none last:pb-0" v-if="playerStore.currentSong?.path">
            <span class="text-[13px] text-body-muted shrink-0 w-15">路径</span>
            <span class="text-sm text-ink text-right break-all truncate max-w-[calc(100%-70px)]" :title="playerStore.currentSong.path">{{ playerStore.currentSong.path }}</span>
          </div>
        </div>
        <div class="mt-6 flex justify-center">
          <button class="bg-white text-black border-none py-2 px-8 rounded-xl text-sm font-medium cursor-pointer transition-all duration-150 hover:bg-[#f0f0f2] hover:scale-[1.02] active:scale-[0.98]" @click="showInfoModal = false">关闭</button>
        </div>
      </div>
    </n-modal>

    <!-- 删除确认弹窗 -->
    <n-modal v-model:show="showDeleteConfirmModal" preset="dialog" title="确认物理删除"
      content="将从磁盘中永久删除该音频文件及关联缓存，此操作不可恢复。确定删除吗？" positive-text="确定删除" negative-text="取消"
      @positive-click="handleConfirmDelete" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onBeforeUpdate } from 'vue'
import { usePlayerStore } from '../stores/player'
import { useFavoritesStore } from '../stores/favorites'
import { useSystemStore } from '../stores/system'
import { NSlider, NDropdown, useMessage, NModal } from 'naive-ui'
import { wsClient } from '../api/ws'
import { musicDB } from '../utils/indexedDB'
import { getApiUrl } from '../utils/path'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits(['close'])

const playerStore = usePlayerStore()
const favoritesStore = useFavoritesStore()
const systemStore = useSystemStore()
const message = useMessage()

watch(() => props.show, async (newShow) => {
  if (newShow) {
    if (playerStore.playlist.length === 0) {
      console.log('[FullPlayerOverlay] 播放队列为空，尝试自动填充本地歌曲...')
      let localSongs = systemStore.songs || []
      if (localSongs.length === 0) {
        const cached = localStorage.getItem('2fmusic_playlist')
        if (cached) {
          try {
            localSongs = JSON.parse(cached)
          } catch (e) {}
        }
      }
      if (localSongs.length === 0) {
        try {
          await systemStore.fetchSongs()
          localSongs = systemStore.songs || []
        } catch (e) {}
      }
      if (localSongs.length > 0) {
        playerStore.playlist = [...localSongs]
        if (!playerStore.currentSong) {
          const firstSong = localSongs[0]
          playerStore.playSong(firstSong, playerStore.playlist)
        }
      }
    }
  }
})

// 逐字歌词字级时值结构
interface YrcWord {
  text: string
  startTime: number  // 相对于歌曲起点的绝对毫秒
  duration: number   // 持续时间（毫秒）
}

// 歌词行类型
interface LyricLine {
  time: number
  duration?: number  // 整行持续时间（毫秒）
  lines: string[]    // lines[0]=原文, lines[1]=翻译（可选）
  isYrc?: boolean    // 是否是逐字歌词
  words?: YrcWord[]  // 逐字数组
}

const rawLyrics = ref('')
const lyricLines = ref<LyricLine[]>([])
const currentLyricIndex = ref(-1)

const lyricElements = ref<HTMLElement[]>([])

onBeforeUpdate(() => {
  lyricElements.value = []
})



// 加载歌曲歌词
const loadLyricsForSong = async (song: any, skipCache: boolean = false) => {
  if (!song) {
    rawLyrics.value = ''
    lyricLines.value = []
    currentLyricIndex.value = -1
    playerStore.updateLyric('')
    return
  }

  try {
    playerStore.updateLyric('')
    const cacheEnabled = localStorage.getItem('2fmusic_cache_lyrics') === 'true'
    if (cacheEnabled && !skipCache) {
      const cachedLyrics = await musicDB.getLyrics(song.id)
      if (cachedLyrics) {
        rawLyrics.value = cachedLyrics
        parseLyrics(cachedLyrics)
        currentLyricIndex.value = -1
        return
      }
    }

    const data = await wsClient.sendRequest('music/lyrics', {
      title: song.title,
      artist: song.artist,
      album: song.album,
      filename: song.filename,
      song_id: song.id,
      yrc: true
    })

    if (data && data.lyrics) {
      rawLyrics.value = data.lyrics
      parseLyrics(data.lyrics)

      if (cacheEnabled) {
        musicDB.saveLyrics(song.id, data.lyrics).catch(err => {
          console.debug('保存歌词到 IndexedDB 失败:', err)
        })
      }
    } else {
      rawLyrics.value = ''
      lyricLines.value = []
    }
  } catch (e) {
    console.error('加载歌词失败:', e)
    rawLyrics.value = ''
    lyricLines.value = []
  }
  currentLyricIndex.value = -1
}

// 监听歌曲变化，拉取歌词
watch(() => playerStore.currentSong?.id, async (newId) => {
  if (!newId) {
    rawLyrics.value = ''
    lyricLines.value = []
    currentLyricIndex.value = -1
    return
  }
  await loadLyricsForSong(playerStore.currentSong)
}, { immediate: true })

// 解析 LRC 格式和网易云 YRC 格式歌词
const parseLyrics = (lrc: string) => {
  if (!lrc || lrc.trim() === '') {
    lyricLines.value = []
    return
  }

  const lines = lrc.split('\n')
  const timeRegex = /\[(\d{2}):(\d{2})(?:\.(\d{2,3}))?\]/
  const yrcRowRegex = /^\[(\d+),(\d+)\]/
  const yrcWordRegex = /\((\d+),(\d+)(?:,\d+)?\)([^\(]+)/g

  // 结构化时间轴 Map 存储对象
  interface MapValue {
    lines: string[]
    duration?: number
    isYrc?: boolean
    words?: YrcWord[]
  }
  const timeMap = new Map<number, MapValue>()

  lines.forEach(rawLine => {
    const line = rawLine.trim()
    if (!line) return

    if (line.startsWith('{')) {
      try {
        const json = JSON.parse(line)
        if (typeof json.t === 'number') {
          const time = json.t / 1000
          let text = ''
          if (Array.isArray(json.c)) {
            text = (json.c as Array<{ tx?: string }>).map(item => item.tx || '').join('').trim()
          }
          if (text) {
            if (!timeMap.has(time)) {
              timeMap.set(time, { lines: [], isYrc: false })
            }
            timeMap.get(time)!.lines.push(text)
          }
          return
        }
      } catch (e) { /* ignore json parse error */ }
    }

    const yrcMatch = yrcRowRegex.exec(line)
    if (yrcMatch) {
      const timeMs = parseInt(yrcMatch[1])
      const durationMs = parseInt(yrcMatch[2])
      const time = timeMs / 1000

      const content = line.replace(yrcRowRegex, '')
      const words: YrcWord[] = []
      let match;
      let text = ''

      yrcWordRegex.lastIndex = 0
      while ((match = yrcWordRegex.exec(content)) !== null) {
        const wStart = parseInt(match[1])
        const wDuration = parseInt(match[2])
        const wText = match[3]
        words.push({
          text: wText,
          startTime: wStart,
          duration: wDuration
        })
        text += wText
      }

      if (text) {
        if (!timeMap.has(time)) {
          timeMap.set(time, { lines: [], duration: durationMs, isYrc: true, words })
        }
        timeMap.get(time)!.lines.push(text)
      }
      return
    }

    const match = timeRegex.exec(line)
    if (match) {
      const min = parseInt(match[1])
      const sec = parseInt(match[2])
      const msRaw = match[3] ? parseInt(match[3]) : 0
      const time = min * 60 + sec + (msRaw / (match[3] && match[3].length === 3 ? 1000 : 100))
      const text = line.replace(timeRegex, '').trim()
      if (text) {
        if (!timeMap.has(time)) {
          timeMap.set(time, { lines: [], isYrc: false })
        }
        timeMap.get(time)!.lines.push(text)
      }
    } else {
      const isMetadata = /^\[(id|ar|ti|by|hash|al|sign|qq|total|offset|length|re|ve):.*?\]$/i.test(line)
      if (!line.startsWith('{') && !isMetadata) {
        if (!timeMap.has(0)) {
          timeMap.set(0, { lines: [], isYrc: false })
        }
        timeMap.get(0)!.lines.push(line)
      }
    }
  })

  lyricLines.value = Array.from(timeMap.entries())
    .map(([time, val]) => ({
      time,
      duration: val.duration,
      lines: val.lines,
      isYrc: val.isYrc,
      words: val.words
    }))
    .sort((a, b) => a.time - b.time)
}

watch(() => playerStore.currentTime, (time) => {
  if (lyricLines.value.length === 0) {
    if (playerStore.currentLyric) {
      playerStore.updateLyric('')
    }
    return
  }

  let targetIndex = -1
  for (let i = 0; i < lyricLines.value.length; i++) {
    if (time >= lyricLines.value[i].time) {
      targetIndex = i
    } else {
      break
    }
  }

  if (targetIndex !== currentLyricIndex.value) {
    currentLyricIndex.value = targetIndex
    scrollToActiveLyric()

    // 动态同步推送歌词至系统控制卡片/锁屏
    const activeLine = lyricLines.value[targetIndex]
    if (activeLine) {
      const texts = getLineTexts(activeLine)
      playerStore.updateLyric(texts.join(' | '))
    } else {
      playerStore.updateLyric('')
    }
  }
})

const scrollToActiveLyric = () => {
  nextTick(() => {
    if (currentLyricIndex.value === -1 || lyricElements.value.length === 0) return
    const el = lyricElements.value[currentLyricIndex.value]
    if (el) {
      el.scrollIntoView({
        behavior: 'smooth',
        block: 'center'
      })
    }
  })
}

const seekToLyric = (time: number) => {
  playerStore.seek(time)
}

const sliderTime = ref(0)
watch(() => playerStore.currentTime, (val) => {
  sliderTime.value = val
})

const onSliderChange = (val: number) => {
  playerStore.seek(val)
}

// 收藏状态
const isFavorited = computed(() => {
  if (!playerStore.currentSong) return false
  return favoritesStore.favoriteSongIds.includes(playerStore.currentSong.id)
})

const toggleFavorite = async () => {
  if (!playerStore.currentSong) return
  const song = playerStore.currentSong
  if (isFavorited.value) {
    await favoritesStore.removeFavorite([song.id], ['default'])
  } else {
    await favoritesStore.addFavorite(
      [song.id],
      ['default'],
      { [song.id]: { title: song.title, artist: song.artist } }
    )
  }
}

// 三点菜单选项
const showInfoModal = ref(false)
const showDeleteConfirmModal = ref(false)

// 格式化文件大小
const formatSize = (bytes: number) => {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}


const dropdownOptions = computed(() => {
  const song = playerStore.currentSong
  if (!song) return []

  return [
    { label: '歌曲详情', key: 'song_info' },
    { label: '下载歌曲', key: 'download_song' },
    { label: '重新刮削', key: 'clear_cache' },
    { label: '删除歌曲', key: 'delete_song', props: { style: 'color: #ff2d55;' } }
  ]
})

// 菜单项选择处理
const handleMenuSelect = async (key: string) => {
  const song = playerStore.currentSong
  if (!song) return

  if (key === 'clear_cache') {
    const msgInstance = message.loading('正在清除元数据缓存并重新刮削，请稍候...', { duration: 0 })
    try {
      const res = await systemStore.clearMetadata(song.id)
      if (res.success) {
        // 先清空界面上的歌词和封面，呈现正在刮削的过渡效果
        if (playerStore.currentSong && playerStore.currentSong.id === song.id) {
          playerStore.currentSong.album_art = ''
        }
        rawLyrics.value = ''
        lyricLines.value = []
        currentLyricIndex.value = -1

        // 同步请求网上最新重新刮削出的歌词和封面
        await loadLyricsForSong(song, true)
        await playerStore.fetchAlbumArt(song)

        msgInstance.destroy()
        message.success('已重新刮削并成功获取最新歌词与封面')
      } else {
        msgInstance.destroy()
        message.error('清除元数据缓存失败')
      }
    } catch (err) {
      msgInstance.destroy()
      console.error('重新刮削失败:', err)
      message.error('刮削过程发生异常，请重试')
    }
  } else if (key === 'song_info') {
    showInfoModal.value = true
  } else if (key === 'download_song') {
    try {
      message.info('正在准备下载歌曲，请稍候...')
      const url = getApiUrl(`/api/music/play/${song.id}`)
      const response = await fetch(url)
      if (!response.ok) throw new Error('网络流拉取失败')
      const blob = await response.blob()
      const blobUrl = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = blobUrl
      const ext = song.filename ? song.filename.substring(song.filename.lastIndexOf('.')) : '.mp3'
      a.download = `${song.title} - ${song.artist}${ext}`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(blobUrl)
      message.success('已成功保存歌曲')
    } catch (err) {
      console.error('下载歌曲失败:', err)
      message.error('下载歌曲失败')
    }
  } else if (key === 'delete_song') {
    showDeleteConfirmModal.value = true
  }
}

// 确认物理删除
const handleConfirmDelete = async () => {
  const song = playerStore.currentSong
  if (!song) return

  try {
    // 1. 在播放器中清理出当前队列
    playerStore.playlist = playerStore.playlist.filter(s => s.id !== song.id)
    playerStore.queue = playerStore.queue.filter(s => s.id !== song.id)

    // 2. 根据情况切歌或暂停
    if (playerStore.playlist.length > 0) {
      playerStore.next()
    } else {
      if (playerStore.isPlaying) {
        playerStore.togglePlay()
      }
      playerStore.currentSong = null
      playerStore.isPlaying = false
      emit('close')
    }

    // 3. 执行系统级物理删除
    const res = await systemStore.deleteSong(song.id)
    if (res.success) {
      message.success('歌曲文件已从磁盘永久删除')
    } else {
      message.error(res.error || '删除失败')
    }
  } catch (err) {
    console.error('删除歌曲失败:', err)
    message.error('删除歌曲失败')
  }
}

// 播放模式切换
const togglePlayMode = () => {
  const modes: ('list' | 'single' | 'random')[] = ['list', 'single', 'random']
  const nextIdx = (modes.indexOf(playerStore.playMode) + 1) % modes.length
  playerStore.playMode = modes[nextIdx]

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

// 格式化秒数
const formatTime = (secs: number) => {
  if (isNaN(secs)) return '0:00'
  const m = Math.floor(secs / 60)
  const s = Math.floor(secs % 60)
  return `${m}:${s < 10 ? '0' : ''}${s}`
}

// 获取歌词行文本（支持 " / " 或 " | " 分隔的双语歌词）
const getLineTexts = (line: LyricLine): string[] => {
  if (!line || !line.lines || line.lines.length === 0) return []
  if (line.lines.length > 1) {
    return line.lines
  }
  const text = line.lines[0]
  if (text.includes(' / ')) {
    return text.split(' / ').map(p => p.trim()).filter(Boolean)
  }
  if (text.includes(' | ')) {
    return text.split(' | ').map(p => p.trim()).filter(Boolean)
  }
  return [text]
}

// 获取当前字的播放状态类名
const getWordClass = (word: YrcWord) => {
  const curTimeMs = playerStore.currentTime * 1000
  if (curTimeMs >= word.startTime + word.duration) {
    return 'is-played'
  } else if (curTimeMs >= word.startTime) {
    return 'is-playing'
  }
  return 'is-pending'
}

// 计算当前字的染色高亮百分比 (利用 CSS 硬件加速过渡避免 timeupdate 卡顿)
const getWordStyle = (word: YrcWord) => {
  const curTimeMs = playerStore.currentTime * 1000
  if (curTimeMs >= word.startTime && curTimeMs < word.startTime + word.duration) {
    const remainingTime = Math.max(0, word.startTime + word.duration - curTimeMs)
    return {
      'transition': `background-size ${remainingTime}ms linear`
    }
  }
  return {}
}
</script>

<style scoped>
.yrc-word {
  background: linear-gradient(to right, #ffffff, #ffffff) no-repeat;
  background-size: 0% 100%;
  background-color: rgba(255, 255, 255, 0.45);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  color: transparent !important;
  display: inline;
}

.yrc-word.is-played {
  background-size: 100% 100% !important;
  transition: none !important;
}

.yrc-word.is-playing {
  background-size: 100% 100%;
  /* transition 将在 HTML 中根据具体 duration 动态配置 */
}

.yrc-word.is-pending {
  background-size: 0% 100% !important;
  transition: none !important;
}

.segmented-control {
  display: flex;
  position: relative;
  border-radius: 9999px;
  background-color: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 2px;
}

.segmented-slider {
  position: absolute;
  top: 2px;
  bottom: 2px;
  left: 2px;
  border-radius: 9999px;
  background-color: rgba(255, 255, 255, 0.12);
  transition: transform 0.3s cubic-bezier(0.25, 1, 0.5, 1);
  pointer-events: none;
}

.segmented-control button {
  width: 80px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: rgba(255, 255, 255, 0.55);
  font-size: 11px;
  font-weight: 500;
  border-radius: 9999px;
  cursor: pointer;
  transition: color 0.2s ease;
  white-space: nowrap;
  outline: none;
  padding: 0;
  margin: 0;
  z-index: 1;
}

.segmented-control button.active {
  color: #ffffff;
  font-weight: 600;
}
</style>

