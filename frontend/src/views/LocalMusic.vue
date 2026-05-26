<template>
  <div class="flex flex-col flex-1 min-h-0 local-music-view">
    <!-- 头部操作区 (重构为 Tailwind 极简原子类) -->
    <div class="view-header flex justify-between items-center mb-6 max-md:flex-col max-md:items-stretch max-md:gap-3 max-md:mb-4">
      <div class="flex items-center gap-3 w-full">
        <n-button v-if="playlistId" circle secondary class="mr-2" @click="goBackToFavorites" title="返回收藏夹">
          <template #icon>
            <SvgIcon name="chevron-left" />
          </template>
        </n-button>
        <h1 class="view-title">{{ pageTitle }}</h1>
        <span class="text-sm text-body-muted font-normal" v-if="filteredSongs.length > 0">
          ({{ filteredSongs.length }} 首)
        </span>
      </div>

      <div class="flex items-center gap-3 max-md:w-full max-md:justify-between max-md:gap-2">
        <!-- 搜索 -->
        <n-input v-model:value="searchQuery" placeholder="搜索标题、歌手、专辑..." round clearable class="w-80! max-md:flex-1 max-md:w-full">
          <template #prefix>
            <SvgIcon name="search" class="mr-1.5" />
          </template>
        </n-input>

        <!-- 排序（仅本地音乐显示） -->
        <div v-if="!playlistId" class="flex items-center gap-1">
          <n-dropdown trigger="click" :options="sortOptions" @select="handleSortSelect">
            <n-button round>
              <template #icon>
                <SvgIcon :name="sortOrder === 'asc' ? 'sort-amount-down' : 'sort-amount-up'" />
              </template>
              {{ currentSortLabel }}
            </n-button>
          </n-dropdown>
        </div>
      </div>
    </div>

    <!-- 批量管理工具条 (直接套用 components.css 全新毛玻璃 batch-toolbar 底座) -->
    <div v-if="isBatchMode" class="batch-toolbar">
      <div class="batch-left">
        <n-checkbox :checked="isAllSelected" :indeterminate="isSomeSelected" @update:checked="toggleSelectAll">
          已选 {{ selectedSongIds.size }} 首
        </n-checkbox>
      </div>
      <div class="batch-right">
        <n-dropdown v-if="!playlistId" trigger="click" :options="batchPlaylistOptions"
          @select="handleBatchAddToPlaylist">
          <n-button round secondary :disabled="selectedSongIds.size === 0">
            <template #icon>
              <SvgIcon name="plus" />
            </template> 添加到收藏夹
          </n-button>
        </n-dropdown>

        <n-button v-else round type="error" secondary :disabled="selectedSongIds.size === 0"
          @click="handleBatchRemoveFromPlaylist">
          <template #icon>
            <SvgIcon name="trash" />
          </template> 从收藏夹移除
        </n-button>

        <n-button v-if="!playlistId" round type="error" secondary :disabled="selectedSongIds.size === 0"
          @click="showBatchDeleteModal = true">
          <template #icon>
            <SvgIcon name="trash" />
          </template> 物理删除
        </n-button>

        <n-button class="clear-selection-btn" round text :disabled="selectedSongIds.size === 0" @click="clearSelection">
          取消选择
        </n-button>
        <n-button class="exit-batch-btn" round text @click="toggleBatchMode">
          退出管理
        </n-button>
        <n-button class="mobile-exit-btn" circle secondary @click="toggleBatchMode" title="退出管理">
          <template #icon>
            <SvgIcon name="close" />
          </template>
        </n-button>
      </div>
    </div>

    <!-- 歌曲列表 -->
    <div class="song-list-container">
      <div v-if="isLoading" class="flex flex-col items-center justify-center gap-3 py-15">
        <n-spin :size="32" />
        <span class="text-xs text-body-muted">正在加载歌曲...</span>
      </div>

      <div v-else-if="filteredSongs.length === 0" class="empty-state">
        <SvgIcon name="music" />
        <h3>暂无音乐</h3>
        <p v-if="searchQuery">没有匹配的歌曲，请尝试其他关键词</p>
        <p v-else-if="playlistId">当前收藏夹为空，可在“本地音乐”列表中右键加入此收藏夹</p>
        <p v-else>音乐库为空，请前往“目录管理”添加挂载路径或前往“网易下载”获取</p>
      </div>

      <template v-else>
        <!-- 固定在顶部的极简表头 -->
        <div class="song-grid-header">
          <div v-if="isBatchMode" class="col-checkbox"></div>
          <div class="col-title">标题</div>
          <div class="col-artist">歌手</div>
          <div class="col-album">专辑</div>
          <div class="col-size">大小</div>
          <div class="col-actions"></div>
        </div>

        <!-- 高性能虚拟列表滚动区 -->
        <n-virtual-list class="song-list-scroll-area virtual-list-container" :item-size="64"
          :items="filteredSongs" key-field="id" :item-resizable="false">
          <template #default="{ item: song }">
            <div class="song-row" :class="{
              playing: playerStore.currentSong?.id === song.id,
              selected: selectedSongIds.has(song.id)
            }" @click="handleRowClick(song)" @dblclick="playSong(song)">
              <!-- 复选框 -->
              <div v-if="isBatchMode" class="col-checkbox" @click.stop>
                <n-checkbox :checked="selectedSongIds.has(song.id)"
                  @update:checked="(val) => toggleSongSelection(song.id, val)" />
              </div>

              <!-- 标题（含封面、播放状态指示） -->
              <div class="col-title">
                <div class="cover-box" @click.stop="handlePlayBtnClick(song)">
                  <img v-cached-src="{ id: song.id, src: song.album_art }" loading="lazy" alt="Cover" />
                  <div class="play-hover">
                    <SvgIcon
                      :name="playerStore.currentSong?.id === song.id && playerStore.isPlaying ? 'pause' : 'play'" />
                  </div>
                </div>
                <div class="title-text-box">
                  <span class="song-name">{{ song.title }}</span>
                  <span v-if="playerStore.currentSong?.id === song.id" class="playing-gif">
                    <SvgIcon name="volume-up" />
                  </span>
                </div>
              </div>

              <!-- 歌手 -->
              <div class="col-artist" :title="song.artist">{{ song.artist }}</div>

              <!-- 专辑 -->
              <div class="col-album" :title="song.album">{{ song.album || '-' }}</div>

              <!-- 大小 -->
              <div class="col-size">{{ formatSize(song.size) }}</div>

              <!-- 右侧操作 -->
              <div class="col-actions" @click.stop>
                <n-dropdown trigger="click" :options="getRowDropdownOptions(song)"
                  @select="(key) => handleRowAction(key, song)">
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

    <!-- 物理删除确认模态框 -->
    <n-modal v-model:show="showDeleteConfirm" preset="dialog">
      <template #header>
        <div>确认删除歌曲</div>
      </template>
      <div>
        确定要从服务器磁盘上<strong>永久物理删除</strong>歌曲《{{ songToDelete?.title }}》吗？此操作不可撤销，文件及关联封面、歌词都将被清理。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showDeleteConfirm = false">取消</n-button>
          <n-button round type="error" @click="confirmDeleteSong">确认物理删除</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 批量物理删除确认 -->
    <n-modal v-model:show="showBatchDeleteModal" preset="dialog">
      <template #header>
        <div>确认批量物理删除</div>
      </template>
      <div>
        确定要<strong>物理删除</strong>选中的 {{ selectedSongIds.size }} 首歌曲吗？此操作将永久抹除这些磁盘文件，无法恢复。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showBatchDeleteModal = false">取消</n-button>
          <n-button round type="error" @click="confirmBatchDelete">确认批量物理删除</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSystemStore } from '../stores/system'
import { usePlayerStore } from '../stores/player'
import { useFavoritesStore } from '../stores/favorites'
import { NDropdown, NCheckbox, NModal, NButton, NInput, NSpace, NVirtualList, NSpin, useMessage } from 'naive-ui'
import type { Song } from '../types'
import SvgIcon from '../components/SvgIcon.vue'

const route = useRoute()
const router = useRouter()
const systemStore = useSystemStore()
const playerStore = usePlayerStore()
const favoritesStore = useFavoritesStore()
const message = useMessage()

const searchQuery = ref('')
const isLoading = ref(false)

// 路由参数：如果存在 id，说明是某收藏夹详情
const playlistId = computed(() => route.params.id as string)

// 收藏夹元数据
const currentPlaylist = computed(() => {
  if (!playlistId.value) return null
  return favoritesStore.playlists.find(p => String(p.id) === String(playlistId.value))
})

const pageTitle = computed(() => {
  if (playlistId.value) {
    return currentPlaylist.value?.name || '收藏夹详情'
  }
  return '本地音乐'
})

// 排序状态（本地音乐专属）
const currentSort = ref<'title' | 'artist' | 'album' | 'size' | 'mtime' | 'playCount'>('title')
const sortOrder = ref<'asc' | 'desc'>('asc')

const sortOptions = [
  { label: '按标题排序', key: 'title' },
  { label: '按歌手排序', key: 'artist' },
  { label: '按专辑排序', key: 'album' },
  { label: '按大小排序', key: 'size' },
  { label: '按时间排序', key: 'mtime' }
]

const currentSortLabel = computed(() => {
  const map = {
    title: '标题排序',
    artist: '歌手排序',
    album: '专辑排序',
    size: '文件大小',
    mtime: '入库时间',
    playCount: '播放频率'
  }
  return map[currentSort.value] || '排序'
})

const handleSortSelect = (key: 'title' | 'artist' | 'album' | 'size' | 'mtime') => {
  if (currentSort.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    currentSort.value = key
    sortOrder.value = 'asc'
  }
  saveSortState()
}

const saveSortState = () => {
  const saved = localStorage.getItem('2fmusic_state')
  const state = saved ? JSON.parse(saved) : {}
  state.currentSort = currentSort.value
  state.sortOrder = sortOrder.value
  localStorage.setItem('2fmusic_state', JSON.stringify(state))
}

// 批量管理
const isBatchMode = ref(false)
const selectedSongIds = ref<Set<string>>(new Set())

const toggleBatchMode = () => {
  isBatchMode.value = !isBatchMode.value
  if (!isBatchMode.value) {
    selectedSongIds.value.clear()
  }
}

const toggleSongSelection = (id: string, val: boolean) => {
  if (val) {
    selectedSongIds.value.add(id)
  } else {
    selectedSongIds.value.delete(id)
  }
}

const isAllSelected = computed(() => {
  return filteredSongs.value.length > 0 && selectedSongIds.value.size === filteredSongs.value.length
})

const isSomeSelected = computed(() => {
  return selectedSongIds.value.size > 0 && selectedSongIds.value.size < filteredSongs.value.length
})

const toggleSelectAll = (checked: boolean) => {
  if (checked) {
    filteredSongs.value.forEach(s => selectedSongIds.value.add(s.id))
  } else {
    selectedSongIds.value.clear()
  }
}

const clearSelection = () => {
  selectedSongIds.value.clear()
}

// 歌曲数据源（基于路由过滤）
const currentSongsSource = computed(() => {
  if (playlistId.value) {
    // 收藏夹歌曲列表：过滤出 favoritesStore.favoriteSongIds 里的歌曲
    return systemStore.songs.filter(s => favoritesStore.favoriteSongIds.includes(s.id))
  }
  return systemStore.songs
})

// 根据搜索及排序过滤当前歌曲
const filteredSongs = computed(() => {
  let list = [...currentSongsSource.value]

  // 1. 过滤搜索
  const query = searchQuery.value.trim().toLowerCase()
  if (query) {
    list = list.filter(
      s =>
        s.title.toLowerCase().includes(query) ||
        s.artist.toLowerCase().includes(query) ||
        s.album.toLowerCase().includes(query)
    )
  }

  // 2. 排序（收藏夹页面不进行后台排序，按收藏添加时间显示）
  if (!playlistId.value) {
    list.sort((a, b) => {
      let valA: any = a[currentSort.value] || ''
      let valB: any = b[currentSort.value] || ''

      if (typeof valA === 'string') {
        valA = valA.toLowerCase()
        valB = valB.toLowerCase()
      }

      const order = sortOrder.value === 'asc' ? 1 : -1

      if (valA > valB) return order
      if (valA < valB) return -order
      return 0
    })
  }

  return list
})

// 加载数据
const loadData = async () => {
  // 如果之前已经有加载过的歌曲数据和歌单数据，直接利用缓存渲染，避免展示 Loading 转圈造成导航切换卡顿
  const hasCache = systemStore.songs.length > 0 && favoritesStore.playlists.length > 0
  if (!hasCache) {
    isLoading.value = true
  }

  try {
    // 采用 Promise.all 并发请求，消除串行请求的累加等待时间
    await Promise.all([
      systemStore.fetchSongs(),
      favoritesStore.fetchPlaylists()
    ])

    // 如果是收藏夹详情页，也静默拉取其内的关联歌曲 ID 关系
    if (playlistId.value) {
      await favoritesStore.fetchPlaylistSongs(playlistId.value)
    }
  } catch (err) {
    console.error('Failed to sync library data:', err)
  } finally {
    isLoading.value = false
  }
}

onMounted(() => {
  // 从本地存储恢复排序设置
  try {
    const saved = localStorage.getItem('2fmusic_state')
    if (saved) {
      const state = JSON.parse(saved)
      if (state.currentSort) currentSort.value = state.currentSort
      if (state.sortOrder) sortOrder.value = state.sortOrder
    }
  } catch (e) {
    console.error(e)
  }

  loadData()
})

// 监听路由改变，重新拉取收藏夹歌曲ID
watch(() => playlistId.value, async (newVal) => {
  isBatchMode.value = false
  selectedSongIds.value.clear()
  if (newVal) {
    isLoading.value = true
    await favoritesStore.fetchPlaylistSongs(newVal)
    isLoading.value = false
  }
})

// 行点击：多选模式下直接勾选，常规模式下仅高亮或双击播放（移动端下单触直接播放）
const handleRowClick = (song: Song) => {
  if (isBatchMode.value) {
    const id = song.id
    toggleSongSelection(id, !selectedSongIds.value.has(id))
  } else {
    // 移动端/触摸屏下，单次点击直接触发播放
    const isMobile = window.innerWidth <= 768
    if (isMobile) {
      playSong(song)
    }
  }
}

// 播放歌曲
const playSong = (song: Song) => {
  playerStore.playSong(song, filteredSongs.value)
}

// 点击封面播放按钮：控制播放/暂停
const handlePlayBtnClick = (song: Song) => {
  if (isBatchMode.value) {
    toggleSongSelection(song.id, !selectedSongIds.value.has(song.id))
    return
  }
  if (playerStore.currentSong?.id === song.id) {
    playerStore.togglePlay()
  } else {
    playSong(song)
  }
}

// 格式化文件大小
const formatSize = (bytes: number) => {
  if (!bytes) return '-'
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(1)} MB`
}

// 返回
const goBackToFavorites = () => {
  router.push('/favorites')
}

// 行右键/操作菜单选项定义
const getRowDropdownOptions = (song: Song) => {
  // 临时读取属性以避免未使用参数报错
  if (song.id) { }
  const options = [
    { label: '播放', key: 'play' },
    { label: '添加至队列', key: 'queue' }
  ]

  if (!playlistId.value) {
    // 本地音乐：支持添加至各收藏夹
    options.push({ label: '添加到收藏夹', key: 'fav' })
  } else {
    // 收藏夹详情：支持移出收藏夹
    options.push({ label: '从收藏夹移出', key: 'remove_fav' })
  }

  options.push({ label: '批量管理', key: 'batch' })

  return options
}

const songToDelete = ref<Song | null>(null)
const showDeleteConfirm = ref(false)
const showBatchDeleteModal = ref(false)

const handleRowAction = async (key: string, song: Song) => {
  if (key === 'play') {
    playSong(song)
  } else if (key === 'queue') {
    playerStore.addToQueue(song)
    message.success('已添加到播放队列')
  } else if (key === 'fav') {
    // 弹窗添加到指定收藏夹
    const res = await favoritesStore.addFavorite(song.id, 'default', song.title, song.artist)
    if (res.success) message.success('已成功添加至默认收藏夹')
    else message.error(res.error)
  } else if (key === 'remove_fav') {
    const res = await favoritesStore.removeFavorite(song.id, playlistId.value)
    if (res.success) message.success('已移出该收藏夹')
    else message.error(res.error)
  } else if (key === 'clear_cache') {
    const res = await systemStore.clearMetadata(song.id)
    if (res.success) message.success('已成功清除该歌曲的本地封面和歌词缓存，等待重新扫描')
    else message.error('清除缓存失败')
  } else if (key === 'delete') {
    songToDelete.value = song
    showDeleteConfirm.value = true
  } else if (key === 'batch') {
    isBatchMode.value = true
    selectedSongIds.value.add(song.id)
  }
}

// 确认删除单曲
const confirmDeleteSong = async () => {
  if (!songToDelete.value) return
  const id = songToDelete.value.id
  const res = await systemStore.deleteSong(id)
  showDeleteConfirm.value = false
  if (res.success) {
    message.success('歌曲文件已从磁盘上彻底物理删除')
    // 如果删除的正是正在播放的歌，停止它
    if (playerStore.currentSong?.id === id) {
      playerStore.currentSong = null
    }
  } else {
    message.error(res.error)
  }
}

// 批量添加至收藏夹选项
const batchPlaylistOptions = computed(() => {
  return favoritesStore.playlists.map(p => ({
    label: p.name,
    key: p.id
  }))
})

// 批量添加操作
const handleBatchAddToPlaylist = async (playlistId: string) => {
  const ids = Array.from(selectedSongIds.value)
  const songsInfo: Record<string, { title: string; artist: string }> = {}
  ids.forEach(id => {
    const song = systemStore.songs.find(s => s.id === id)
    if (song) {
      songsInfo[id] = { title: song.title, artist: song.artist }
    }
  })

  const res = await favoritesStore.batchAddFavorites(ids, [playlistId], songsInfo)
  if (res.success) {
    message.success(`已批量加入 ${ids.length} 首歌曲到收藏夹`)
    isBatchMode.value = false
    selectedSongIds.value.clear()
  } else {
    message.error(res.error)
  }
}

// 从当前收藏夹批量移出
const handleBatchRemoveFromPlaylist = async () => {
  const ids = Array.from(selectedSongIds.value)
  const res = await favoritesStore.batchRemoveFavorites(ids, [playlistId.value])
  if (res.success) {
    message.success(`已批量从收藏夹中移出 ${ids.length} 首歌曲`)
    isBatchMode.value = false
    selectedSongIds.value.clear()
  } else {
    message.error(res.error)
  }
}

// 批量物理删除
const confirmBatchDelete = async () => {
  showBatchDeleteModal.value = false
  const ids = Array.from(selectedSongIds.value)
  let successCount = 0

  for (const id of ids) {
    const res = await systemStore.deleteSong(id)
    if (res.success) {
      successCount++
      if (playerStore.currentSong?.id === id) {
        playerStore.currentSong = null
      }
    }
  }

  message.info(`批量删除完成: 成功删除 ${successCount} 首，失败 ${ids.length - successCount} 首`)
  isBatchMode.value = false
  selectedSongIds.value.clear()
}
</script>

<style scoped>
/* 仅留存为处理虚拟列表所必须的布局锁定与移动端响应式 Grid 重塑 */

.virtual-list-container {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.back-icon-btn {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: 1px solid var(--hairline);
  background-color: var(--canvas);
  color: var(--ink);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color var(--transition-fast), border-color var(--transition-fast);
  flex-shrink: 0;
}

.back-icon-btn:hover {
  background-color: var(--canvas-parchment);
  border-color: var(--body-muted);
}

.back-icon-btn :deep(.svg-icon) {
  font-size: 16px;
  transition: transform var(--transition-fast);
}

.back-icon-btn:hover :deep(.svg-icon) {
  transform: translateX(-2px);
}

/* 移动端专用多选退出按钮，默认在 PC 宽屏端隐藏 */
.batch-right :deep(.mobile-exit-btn) {
  display: none !important;
}

/* 状态页 - 图标尺寸 */
.empty-state :deep(.svg-icon) {
  font-size: 40px;
  color: var(--body-muted);
  opacity: 0.5;
  margin-bottom: 16px;
}

@media (max-width: 768px) {
  .song-grid-header {
    display: none;
  }

  .song-row {
    display: grid !important;
    grid-template-areas:
      "check cover title action"
      "check cover artist action";
    grid-template-columns: auto auto 1fr auto;
    grid-template-rows: auto auto;
    align-items: center;
    padding: 8px 12px;
    gap: 2px 12px;
  }

  .col-checkbox {
    grid-area: check;
    display: flex;
    align-items: center;
    width: auto;
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

  .col-album,
  .col-size {
    display: none !important;
  }

  .col-actions {
    grid-area: action;
    grid-row: span 2;
    display: flex;
    align-items: center;
    width: auto;
  }

  .view-header .header-actions :deep(.n-button) {
    width: 38px;
    height: 38px;
    padding: 0;
    justify-content: center;
    border-radius: 50%;
  }

  .view-header .header-actions :deep(.n-button .n-button__content) {
    display: none !important;
  }

  .view-header .header-actions :deep(.n-button .n-button__icon) {
    margin: 0 !important;
  }

  /* 批量工具栏按钮深度穿透适配与移动端文字隐藏 */
  .batch-right :deep(.n-button:not(.n-button--text-type)) {
    width: 36px;
    height: 36px;
    padding: 0;
    justify-content: center;
    border-radius: 50%;
    min-width: 36px;
    flex-shrink: 0;
  }

  .batch-right :deep(.n-button:not(.n-button--text-type) .n-button__content) {
    display: none !important;
  }

  .batch-right :deep(.n-button:not(.n-button--text-type) .n-button__icon) {
    margin: 0 !important;
  }

  /* 移动端下彻底隐藏 PC 端的“取消选择”与“退出管理”文本按钮 */
  .batch-right :deep(.clear-selection-btn),
  .batch-right :deep(.exit-batch-btn) {
    display: none !important;
  }

  /* 移动端下显式唤醒退出按钮 */
  .batch-right :deep(.mobile-exit-btn) {
    display: inline-flex !important;
  }
}

/* 锁定外层滚动 - 虚拟列表自行处理滚动 */
:global(.main-scroll:has(.local-music-view)) {
  overflow: hidden !important;
  display: flex !important;
  flex-direction: column !important;
  padding-bottom: 1px !important;
}
</style>
