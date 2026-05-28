<template>
  <div class="flex flex-col flex-1 min-h-0 local-music-view">
    <!-- 头部操作区 -->
    <div class="flex justify-between items-center mb-6 max-md:flex-col max-md:items-stretch max-md:gap-3 max-md:mb-4">
      <div class="flex items-center gap-3 w-full">
        <n-button v-if="playlistId" circle secondary class="mr-2" @click="goBackToFavorites" title="返回收藏夹">
          <template #icon>
            <SvgIcon name="chevron-left" />
          </template>
        </n-button>
        <h1 class="font-display text-[28px] font-semibold text-ink m-0 tracking-[-0.5px]">{{ pageTitle }}</h1>
        <span class="text-sm text-body-muted font-normal" v-if="filteredSongs.length > 0">
          ({{ filteredSongs.length }} 首)
        </span>
      </div>

      <div class="flex items-center gap-3 max-md:w-full max-md:justify-between max-md:gap-2">
        <!-- 搜索 -->
        <n-input v-model:value="searchQuery" placeholder="搜索标题、歌手、专辑..." round clearable
          class="w-[clamp(180px,24vw,320px)]! max-md:flex-1 max-md:w-full">
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

    <!-- 批量管理工具条 -->
    <div v-if="isBatchMode"
      class="h-[52px] bg-(--bg-player) border border-(--border-main) backdrop-blur-player rounded-md mb-4 flex items-center justify-between px-4 box-border">
      <div>
        <n-checkbox :checked="isAllSelected" :indeterminate="isSomeSelected" @update:checked="toggleSelectAll">
          已选 {{ selectedSongIds.size }} 首
        </n-checkbox>
      </div>
      <div class="flex items-center gap-2">
        <n-dropdown v-if="!playlistId" trigger="click" :options="batchPlaylistOptions"
          @select="handleBatchAddToPlaylist">
          <n-button round secondary :disabled="selectedSongIds.size === 0"
            class="max-md:w-9 max-md:h-9 max-md:p-0 max-md:rounded-full max-md:min-w-9 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
            <template #icon>
              <SvgIcon name="plus" class="max-md:m-0!" />
            </template>
            <span class="max-md:hidden">添加到收藏夹</span>
          </n-button>
        </n-dropdown>

        <n-button v-else round type="error" secondary :disabled="selectedSongIds.size === 0"
          @click="handleBatchRemoveFromPlaylist"
          class="max-md:w-9 max-md:h-9 max-md:p-0 max-md:rounded-full max-md:min-w-9 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
          <template #icon>
            <SvgIcon name="trash" class="max-md:m-0!" />
          </template>
          <span class="max-md:hidden">从收藏夹移除</span>
        </n-button>

        <n-button v-if="!playlistId" round type="error" secondary :disabled="selectedSongIds.size === 0"
          @click="showBatchDeleteModal = true"
          class="max-md:w-9 max-md:h-9 max-md:p-0 max-md:rounded-full max-md:min-w-9 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
          <template #icon>
            <SvgIcon name="trash" class="max-md:m-0!" />
          </template>
          <span class="max-md:hidden">物理删除</span>
        </n-button>

        <n-button class="max-md:hidden!" round text :disabled="selectedSongIds.size === 0" @click="clearSelection">
          取消选择
        </n-button>
        <n-button class="max-md:hidden!" round text @click="toggleBatchMode">
          退出管理
        </n-button>
        <n-button class="hidden! max-md:inline-flex!" circle secondary @click="toggleBatchMode" title="退出管理">
          <template #icon>
            <SvgIcon name="close" />
          </template>
        </n-button>
      </div>
    </div>

    <!-- 歌曲列表 -->
    <div class="flex flex-col flex-1 min-h-0">
      <div v-if="isLoading" class="flex flex-col items-center justify-center gap-3 py-15">
        <n-spin :size="32" />
        <span class="text-xs text-body-muted">正在加载歌曲...</span>
      </div>

      <div v-else-if="filteredSongs.length === 0" class="flex flex-col items-center justify-center py-16 text-center">
        <SvgIcon name="music" class="text-[36px] text-body-muted mb-4" />
        <h3 class="m-0 mb-2 text-lg font-semibold text-ink">暂无音乐</h3>
        <p class="m-0 text-[13px] text-body-muted" v-if="searchQuery">没有匹配的歌曲，请尝试其他关键词</p>
        <p class="m-0 text-[13px] text-body-muted" v-else-if="playlistId">当前收藏夹为空，可在“本地音乐”列表中右键或点击操作按钮加入此收藏夹</p>
        <p class="m-0 text-[13px] text-body-muted" v-else>音乐库为空，请前往“目录管理”添加挂载路径或前往“网易下载”获取</p>
      </div>

      <template v-else>
        <!-- 固定在顶部的极简表头 -->
        <div
          class="song-grid-header max-md:hidden!"
          :style="{ paddingRight: `${16 + scrollbarWidth}px` }">
          <div v-if="isBatchMode" class="col-check"></div>
          <div class="col-title">标题</div>
          <div class="col-artist">歌手</div>
          <div class="col-album">专辑</div>
          <div class="col-size">大小</div>
          <div class="col-actions"></div>
        </div>

        <!-- 高性能虚拟列表滚动区 -->
        <n-virtual-list ref="virtualListRef" class="flex-1 min-h-0 overflow-hidden" :item-size="64" :items="filteredSongs" key-field="id"
          :item-resizable="false" :ignore-item-resize="true" @resize="updateScrollbarWidth">
          <template #default="{ item: song }">
            <div
              class="group song-row max-md:grid! max-md:grid-cols-[auto_auto_1fr_auto] max-md:grid-rows-[auto_auto] max-md:[grid-template-areas:'check_cover_title_action'_'check_cover_artist_action'] max-md:items-center max-md:p-[8px_12px] max-md:gap-x-3 max-md:gap-y-[2px]"
              :class="{
                'text-(--primary) font-semibold': playerStore.currentSong?.id === song.id,
                'bg-(--primary-alpha-16) backdrop-blur-card border border-(--primary-alpha-10) dark:bg-(--primary-on-dark-alpha-16) dark:border-(--primary-on-dark-alpha-12)': selectedSongIds.has(song.id)
              }" @click="handleRowClick(song)" @dblclick="playSong(song)" @contextmenu.prevent="handleContextMenu($event, song)">
              <!-- 复选框 -->
              <div v-if="isBatchMode"
                class="col-check max-md:[grid-area:check] max-md:w-auto max-md:flex max-md:items-center"
                @click.stop>
                <n-checkbox :checked="selectedSongIds.has(song.id)"
                  @update:checked="(val) => toggleSongSelection(song.id, val)" />
              </div>

              <!-- 标题（含封面、播放状态指示） -->
              <div
                class="col-title max-md:contents!">
                <div
                  class="w-10 h-10 rounded-md overflow-hidden relative shrink-0 max-md:[grid-area:cover] max-md:row-[span_2] max-md:w-10 max-md:h-10"
                  @click.stop="handlePlayBtnClick(song)">
                  <img class="w-full h-full object-cover" v-cached-src="{ id: song.id, src: song.album_art }"
                    loading="lazy" alt="Cover" />
                  <div
                    class="absolute inset-0 bg-black/40 flex items-center justify-center text-white opacity-0 text-xs transition-opacity duration-200 group-hover:opacity-100">
                    <SvgIcon
                      :name="playerStore.currentSong?.id === song.id && playerStore.isPlaying ? 'pause' : 'play'" />
                  </div>
                </div>
                <div
                  class="flex items-center gap-2 overflow-hidden max-md:[grid-area:title] max-md:self-end max-md:overflow-hidden">
                  <span class="text-sm whitespace-nowrap overflow-hidden text-ellipsis">{{ song.title }}</span>
                  <span v-if="playerStore.currentSong?.id === song.id" class="text-[11px] text-(--primary)">
                    <SvgIcon name="volume-up" />
                  </span>
                </div>
              </div>

              <!-- 歌手 -->
              <div
                class="col-artist max-md:[grid-area:artist] max-md:self-start max-md:min-w-0 max-md:text-[12px] max-md:text-body-muted max-md:m-0 max-md:p-0"
                :title="song.artist">{{ song.artist }}</div>

              <!-- 专辑 -->
              <div
                class="col-album max-md:hidden!"
                :title="song.album">{{ song.album || '-' }}</div>

              <!-- 大小 -->
              <div class="col-size max-md:hidden!">{{
                formatSize(song.size) }}</div>

              <!-- 右侧操作 -->
              <div
                class="col-actions max-md:[grid-area:action] max-md:row-[span_2] max-md:w-auto max-md:flex max-md:items-center"
                @click.stop>
                <n-dropdown trigger="click" :options="getRowDropdownOptions(song)"
                  @select="(key) => handleRowAction(key, song)">
                  <n-button circle text :depth="3"
                    class="bg-transparent border-none text-body-muted cursor-pointer opacity-60 w-8 h-8 rounded-full flex items-center justify-center transition-all duration-200 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10">
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

    <!-- 专属右键上下文操作菜单 -->
    <n-dropdown
      trigger="manual"
      placement="bottom-start"
      :show="showDropdown"
      :options="dropdownSong ? getRowDropdownOptions(dropdownSong) : []"
      :x="x"
      :y="y"
      @select="handleDropdownSelect"
      @clickoutside="handleClickOutside"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useSystemStore } from '../stores/system'
import { usePlayerStore } from '../stores/player'
import { useFavoritesStore } from '../stores/favorites'
import { NDropdown, NCheckbox, NModal, NButton, NInput, NSpace, NVirtualList, NSpin, useMessage } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
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

// 路由 id 参数标识收藏夹详情
const playlistId = computed(() => route.params.id as string)

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

// 歌曲数据源（路由过滤）
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

const scrollbarWidth = ref(0)
const virtualListRef = ref<any>(null)

const updateScrollbarWidth = () => {
  if (virtualListRef.value?.$el) {
    const el = virtualListRef.value.$el
    scrollbarWidth.value = el.offsetWidth - el.clientWidth
  }
}

watch(() => filteredSongs.value, () => {
  nextTick(() => {
    setTimeout(updateScrollbarWidth, 50)
  })
}, { deep: true })

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
  setTimeout(updateScrollbarWidth, 200)
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

// 行点击：多选模式勾选，移动端单击播放
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

// 播放按钮：控制播放/暂停
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

const goBackToFavorites = () => {
  router.push('/favorites')
}

// 专属右键上下文菜单控制
const showDropdown = ref(false)
const x = ref(0)
const y = ref(0)
const dropdownSong = ref<Song | null>(null)

const handleContextMenu = (e: MouseEvent, song: Song) => {
  e.preventDefault()
  showDropdown.value = false
  nextTick(() => {
    dropdownSong.value = song
    x.value = e.clientX
    y.value = e.clientY
    showDropdown.value = true
  })
}

const handleClickOutside = () => {
  showDropdown.value = false
}

const handleDropdownSelect = (key: string) => {
  showDropdown.value = false
  if (dropdownSong.value) {
    handleRowAction(key, dropdownSong.value)
  }
}

// 行操作菜单选项定义
const getRowDropdownOptions = (_song: Song) => {
  const options: DropdownOption[] = [
    { label: '播放', key: 'play' },
    { label: '添加至队列', key: 'queue' }
  ]

  if (!playlistId.value) {
    options.push({
      label: '添加到收藏夹',
      key: 'add_to_fav_parent',
      children: favoritesStore.playlists.map(p => ({
        label: p.name,
        key: `fav_${p.id}`
      }))
    })
  } else {
    options.push({ label: '从当前收藏夹移出', key: 'remove_fav' })
    const otherPlaylists = favoritesStore.playlists.filter(p => String(p.id) !== String(playlistId.value))
    if (otherPlaylists.length > 0) {
      options.push({
        label: '移动到其他收藏夹',
        key: 'move_to_fav_parent',
        children: otherPlaylists.map(p => ({
          label: p.name,
          key: `move_${p.id}`
        }))
      })
    }
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
  } else if (key.startsWith('fav_')) {
    const targetPlaylistId = key.substring(4)
    const targetPlaylist = favoritesStore.playlists.find(p => String(p.id) === String(targetPlaylistId))
    const playlistName = targetPlaylist ? targetPlaylist.name : '收藏夹'
    const res = await favoritesStore.addFavorite(song.id, targetPlaylistId, song.title, song.artist)
    if (res.success) message.success(`已成功添加至收藏夹《${playlistName}》`)
    else message.error(res.error)
  } else if (key.startsWith('move_')) {
    const targetPlaylistId = key.substring(5)
    const targetPlaylist = favoritesStore.playlists.find(p => String(p.id) === String(targetPlaylistId))
    const playlistName = targetPlaylist ? targetPlaylist.name : '收藏夹'
    const res = await favoritesStore.batchMoveFavorites([song.id], playlistId.value, targetPlaylistId)
    if (res.success) message.success(`已成功移动至收藏夹《${playlistName}》`)
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
