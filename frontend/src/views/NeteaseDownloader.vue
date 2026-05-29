<template>
  <div class="flex flex-col flex-1 min-h-0 netease-downloader-view">
    <!-- API 未配置时提示前往设置 -->
    <div v-if="!isApiConnected" class="flex justify-center items-center flex-1 py-15 max-sm:py-8">
      <div
        class="w-[min(480px,88vw)] text-center glass-panel p-[48px_32px] box-border flex flex-col items-center gap-4 rounded-2xl">
        <div class="text-[48px] text-body-muted opacity-40 mb-1">
          <SvgIcon name="cloud-off" class="w-12 h-12" />
        </div>
        <h2 class="font-display text-xl font-semibold text-ink m-0 tracking-tight">尚未配置网易云 API</h2>
        <p class="text-[13px] text-body-muted m-0 leading-relaxed max-w-90">需要连接网易云音乐 API 服务才能进行搜索与下载，请前往系统设置页完成配置。
        </p>
        <n-button type="primary" round @click="goToSettings">
          <template #icon>
            <SvgIcon name="settings" />
          </template>
          前往设置
        </n-button>
      </div>
    </div>

    <div v-else class="flex flex-col flex-1 min-h-0">
      <!-- 头部：搜索、标题、用户信息、下载管理按钮 -->
      <header
        class="flex justify-between items-center gap-4 mb-6 flex-wrap max-md:flex-col max-md:items-stretch max-md:gap-3">
        <div class="shrink-0">
          <h1 class="view-title">网易云下载</h1>
        </div>

        <div class="flex items-center gap-3 flex-1 min-w-0 justify-end max-md:flex-wrap">
          <n-input v-model:value="searchQuery" placeholder="搜索关键词或粘贴歌单等链接..." round clearable
            class="w-[clamp(180px,24vw,288px)]! max-md:flex-1 max-md:w-full" @keyup.enter="handleSearch">
            <template #prefix>
              <SvgIcon name="search" class="mr-1 flex items-center" />
            </template>
          </n-input>

          <!-- 浮动任务抽屉按钮 -->
          <n-button circle secondary type="primary"
            :class="{ 'bg-primary! text-white! border-primary!': showTaskDrawer }"
            @click="showTaskDrawer = !showTaskDrawer" title="下载管理" class="relative">
            <template #icon>
              <SvgIcon name="download" />
            </template>
            <span v-if="runningTasksCount > 0"
              class="absolute -top-1 -right-1 bg-danger text-white text-[9px] font-semibold w-4 h-4 rounded-full flex items-center justify-center">
              {{ runningTasksCount }}
            </span>
          </n-button>

          <!-- 账号状态区 -->
          <div v-if="systemStore.neteaseUser.logged_in" class="flex items-center gap-2.5">
            <div class="text-right">
              <div class="text-[13px] font-semibold text-ink">{{ systemStore.neteaseUser.nickname }}</div>
              <div class="text-[10px] text-body-muted font-medium"
                :class="{ 'text-vip! font-semibold': systemStore.neteaseUser.is_vip }">
                {{ systemStore.neteaseUser.is_vip ? 'VIP' : '普通用户' }}
              </div>
            </div>
            <n-dropdown trigger="click" :options="userMenuOptions" @select="handleUserMenuSelect">
              <div class="w-9 h-9 rounded-full overflow-hidden cursor-pointer product-cover-shadow">
                <img :src="systemStore.neteaseUser.avatar" alt="Avatar" class="w-full h-full object-cover" />
              </div>
            </n-dropdown>
          </div>
          <n-button v-else type="primary" round @click="openQrModal">
            <template #icon>
              <SvgIcon name="login" />
            </template> 登录网易云
          </n-button>
        </div>
      </header>

      <!-- 解析结果展示 -->
      <div class="flex-1 flex flex-col min-h-0">
        <div v-if="isSearching"
          class="flex flex-col items-center justify-center py-10 text-center gap-3 text-sm text-body-muted">
          <n-spin size="large" />
          <span>正在云端检索中...</span>
        </div>

        <div v-else-if="songsList.length === 0"
          class="flex flex-col items-center justify-center py-12 text-center text-body-muted gap-3">
          <SvgIcon name="cloud-download" class="text-[40px] opacity-50 mb-4" />
          <h3 class="m-0 text-lg font-semibold text-ink">开始下载</h3>
          <p class="m-0 text-xs text-body-muted max-w-112.5 leading-relaxed">粘贴网易云 APP
            中的歌曲、歌单分享链接，或者直接搜索关键词，下载后文件将自动导入并刮削元数据。</p>
        </div>

        <div v-else class="flex flex-col flex-1 min-h-0">
          <!-- 批量操作头部 -->
          <div class="flex justify-between items-center mb-4 max-md:flex-col max-md:items-stretch max-md:gap-3">
            <div class="text-base font-semibold text-ink">
              <span v-if="parsedResultType === 'recommend'">网易云每日推荐</span>
              <span v-else-if="parsedResultType === 'playlist'">歌单解析结果</span>
              <span v-else>搜索结果</span>
              <span class="text-[13px] font-normal text-body-muted" v-if="parsedPlaylistName"> ({{ parsedPlaylistName
              }})</span>
            </div>
            <div class="flex items-center gap-4 max-md:justify-between max-md:w-full">
              <n-checkbox :checked="isAllSelected" :indeterminate="isSomeSelected" @update:checked="toggleSelectAll">
                已选 {{ selectedSongs.size }} 首
              </n-checkbox>
              <n-button round :disabled="selectedSongs.size === 0" @click="downloadSelectedSongs" class="shrink-0">
                <template #icon>
                  <SvgIcon name="download" />
                </template> 下载选中歌曲
              </n-button>
            </div>
          </div>

          <div class="flex flex-col flex-1 min-h-0 overflow-hidden">
            <div class="song-grid-header max-md:hidden" :style="{ paddingRight: `${16 + scrollbarWidth}px` }">
              <div class="col-check"></div>
              <div class="col-title">标题</div>
              <div class="col-artist">歌手</div>
              <div class="col-album">专辑</div>
              <div class="col-quality">音质</div>
              <div class="col-actions"></div>
            </div>

            <n-virtual-list ref="virtualListRef" class="song-list-scroll-area flex-1 min-h-0 overflow-hidden" :item-size="60"
              :items="songsList" key-field="id" :item-resizable="false" :ignore-item-resize="true" @resize="updateScrollbarWidth">
              <template #default="{ item: song }">
                <div class="song-row max-md:grid! max-md:grid-cols-[auto_auto_1fr_auto] max-md:grid-rows-[auto_auto] max-md:[grid-template-areas:'check_cover_title_action'_'check_cover_artist_action'] max-md:items-center max-md:p-[8px_12px] max-md:gap-x-3 max-md:gap-y-[2px]"
                  :class="{ selected: selectedSongs.has(song.id), 'bg-primary/5!': selectedSongs.has(song.id) }"
                  @click="toggleSongSelection(song)">
                  <div class="col-check max-md:[grid-area:check] max-md:w-auto max-md:flex max-md:items-center" @click.stop>
                    <n-checkbox :checked="selectedSongs.has(song.id)"
                      @update:checked="(val) => handleCheckboxChange(song, val)" />
                  </div>

                  <div class="col-title max-md:contents!">
                    <div class="cover-box max-md:[grid-area:cover] max-md:row-[span_2] max-md:w-9 max-md:h-9">
                      <img class="w-full h-full object-cover" v-cached-src="{ id: song.id, src: song.cover }" loading="lazy" alt="Cover" />
                    </div>
                    <div class="flex items-center gap-1.5 overflow-hidden max-md:[grid-area:title] max-md:self-end max-md:overflow-hidden">
                      <span class="song-name">{{ song.title }}</span>
                      <span v-if="song.is_vip" class="text-[9px] font-semibold text-white bg-vip px-1 py-0.5 rounded-[3px] shrink-0">VIP</span>
                    </div>
                  </div>

                  <div class="col-artist max-md:[grid-area:artist] max-md:self-start max-md:min-w-0 max-md:text-[12px] max-md:text-body-muted max-md:m-0 max-md:p-0" :title="song.artist">{{ song.artist }}</div>
                  <div class="col-album max-md:hidden!" :title="song.album">{{ song.album || '-' }}</div>

                  <div class="col-quality max-md:hidden!">
                    <n-tag :bordered="false" size="small" :type="getTagType(song.max_level)">
                      {{ formatQuality(song.max_level) }}
                    </n-tag>
                  </div>

                  <div class="col-actions max-md:[grid-area:action] max-md:row-[span_2] max-md:w-auto max-md:flex max-md:items-center max-md:[&_.sm-round-btn]:w-8 max-md:[&_.sm-round-btn]:h-8 max-md:[&_.sm-round-btn]:p-0 max-md:[&_.sm-round-btn]:justify-center max-md:[&_.sm-round-btn]:rounded-full max-md:[&_.sm-round-btn]:shrink-0 max-md:[&_.sm-round-btn_.n-button__content]:hidden! max-md:[&_.sm-round-btn_.n-button__icon]:m-0!" @click.stop>
                    <n-button class="sm-round-btn" round size="small" type="primary" secondary
                      @click="downloadSingleSong(song)">
                      <template #icon>
                        <SvgIcon name="download" />
                      </template> 下载
                    </n-button>
                  </div>
                </div>
              </template>
            </n-virtual-list>
          </div>
        </div>
      </div>

      <!-- 下载管理任务抽屉 (侧边抽屉) -->
      <n-drawer v-model:show="showTaskDrawer" :width="350" placement="right">
        <n-drawer-content :title="`下载任务 (${totalTasksCount})`" closable>
          <div v-if="totalTasksCount === 0"
            class="flex flex-col items-center justify-center h-full text-body-muted gap-3 py-10">
            <SvgIcon name="tasks" class="text-[32px]" />
            <p class="m-0 text-[13px]">没有正在下载的任务</p>
          </div>
          <div v-else class="flex flex-col gap-4">
            <div v-for="task in sortedTasks" :key="task.task_id"
              class="glass-card p-3 rounded-lg flex flex-col gap-2 box-border border border-border-card"
              :class="task.status">
              <div class="overflow-hidden">
                <div class="text-[12px] font-semibold text-ink truncate" :title="task.title">
                  {{ task.title }}</div>
                <div class="text-[10px] text-body-muted truncate dark:text-white/60" :title="task.artist">{{ task.artist
                }}
                </div>
              </div>

              <!-- 进度及状态 -->
              <div class="flex items-center mt-2">
                <div v-if="task.status === 'downloading' || task.status === 'preparing'"
                  class="flex flex-col gap-1 w-full">
                  <div class="flex justify-between text-[11px] text-body-muted mb-1">
                    <span>{{ task.status === 'preparing' ? '准备中...' : '下载中' }}</span>
                    <span>{{ task.progress }}%</span>
                  </div>
                  <n-progress type="line" :percentage="task.progress" :show-indicator="false" processing size="small" />
                </div>
                <div v-else-if="task.status === 'success'"
                  class="text-[11px] text-success font-semibold flex items-center gap-1">
                  <SvgIcon name="check-circle" /> 成功
                </div>
                <div v-else-if="task.status === 'error'"
                  class="text-[11px] text-danger font-semibold flex items-center gap-1 cursor-help"
                  :title="task.message">
                  <SvgIcon name="exclamation-circle" /> 失败
                </div>
                <div v-else class="text-[10px] text-body-muted">
                  排队中
                </div>
              </div>
            </div>
          </div>
        </n-drawer-content>
      </n-drawer>
    </div>

    <!-- 扫码登录模态框 -->
    <n-modal v-model:show="showQrModal" preset="dialog" title="扫码登录网易云" :closable="true" @after-leave="resetQrState">
      <div class="flex justify-center py-4">
        <div class="flex flex-col items-center gap-4">
          <img v-if="qrImage" :src="qrImage" alt="QR Code"
            class="w-45 h-45 rounded-xl product-cover-shadow" />
          <div v-else
            class="w-45 h-45 flex flex-col items-center justify-center gap-2 text-xs text-body-muted border border-hairline rounded-xl">
            <n-spin :size="28" />
            <span>正在获取二维码...</span>
          </div>
          <div class="text-[13px] text-body-muted text-center font-medium py-1.5 px-4 rounded-full bg-canvas-parchment"
            :class="qrStatusClass">{{ qrStatusText }}</div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useSystemStore } from '../stores/system'
import { NDropdown, NCheckbox, NModal, NButton, NInput, NDrawer, NDrawerContent, NProgress, NSpin, NTag, NVirtualList, useMessage } from 'naive-ui'
import { wsClient } from '../api/ws'
import type { NeteaseSong } from '../types'

const systemStore = useSystemStore()
const router = useRouter()
const message = useMessage()

const goToSettings = () => {
  router.push({ name: 'Settings' })
}

  // 音质级别对应的 tag 颜色
  const getTagType = (level: string) => {
  if (['hires', 'jymaster', 'dolby'].includes(level)) return 'error'
  if (['lossless'].includes(level)) return 'warning'
  if (['exhigh'].includes(level)) return 'info'
  return 'default'
}

// API 配置状态
const isApiConnected = computed(() => {
  return !!systemStore.neteaseConfig.api_base
})

// 用户选项菜单
const userMenuOptions = [
  { label: '退出登录', key: 'logout' }
]

const handleUserMenuSelect = async (key: string) => {
  if (key === 'logout') {
    const res = await systemStore.logoutNetease()
    if (res.success) {
      message.success('已安全退出网易云账号')
    } else {
      message.error('退出登录失败')
    }
  }
}

// 扫码登录逻辑
const showQrModal = ref(false)
const qrImage = ref('')
const qrKey = ref('')
const qrStatus = ref<'waiting' | 'scanned' | 'expired' | 'success' | 'idle'>('idle')
const qrStatusText = ref('加载中...')

const qrStatusClass = computed(() => {
  return {
    scanned: qrStatus.value === 'scanned',
    success: qrStatus.value === 'success',
    expired: qrStatus.value === 'expired'
  }
})

const openQrModal = async () => {
  showQrModal.value = true
  qrImage.value = ''
  qrStatus.value = 'idle'
  qrStatusText.value = '正在加载二维码...'

  try {
    const data = await wsClient.sendRequest('netease/login_qrcode')
    if (data && data.unikey) {
      qrKey.value = data.unikey
      qrImage.value = data.qrimg // Base64 图片
      qrStatus.value = 'waiting'
      qrStatusText.value = '请打开网易云音乐 APP 扫码登录'
    } else {
      qrStatusText.value = '获取二维码失败，请重试'
    }
  } catch (e) {
    qrStatusText.value = '加载失败，请检查 API 是否开启'
  }
}

const resetQrState = () => {
  qrImage.value = ''
  qrKey.value = ''
  qrStatus.value = 'idle'
  qrStatusText.value = ''
}

  // 批量选择
  const selectedSongs = ref<Map<number | string, NeteaseSong>>(new Map())

// 搜索与链接解析
const searchQuery = ref('')
const isSearching = ref(false)
const songsList = ref<NeteaseSong[]>([])
const parsedResultType = ref<'song' | 'playlist' | 'search' | 'recommend'>('search')
const parsedPlaylistName = ref('')

const canLoadRecommend = computed(() => {
  return isApiConnected.value && systemStore.neteaseUser.logged_in
})

const loadRecommendSongs = async () => {
  if (searchQuery.value.trim()) return

  // 判断是否已有推荐缓存
  const hasCache = systemStore.neteaseRecommendSongs.length > 0
  if (!hasCache) {
    isSearching.value = true
    songsList.value = []
  } else {
    // 若有缓存，直接赋给列表渲染，免去转圈等待和页面闪烁
    songsList.value = systemStore.neteaseRecommendSongs
  }

  parsedPlaylistName.value = ''
  selectedSongs.value.clear()
  parsedResultType.value = 'recommend'

  try {
    const data = await systemStore.fetchNeteaseRecommendSongs()
    songsList.value = data || []
  } catch (e: any) {
    console.error('加载网易云每日推荐歌曲失败:', e)
  } finally {
    isSearching.value = false
  }
}

watch(canLoadRecommend, (newVal) => {
  if (newVal) {
    loadRecommendSongs()
  } else {
    if (parsedResultType.value === 'recommend') {
      songsList.value = []
      parsedResultType.value = 'search'
    }
  }
}, { immediate: true })

watch(searchQuery, (newVal) => {
  if (!newVal.trim()) {
    if (canLoadRecommend.value) {
      loadRecommendSongs()
    } else {
      songsList.value = []
      parsedResultType.value = 'search'
    }
  }
})

const handleSearch = async () => {
  const query = searchQuery.value.trim()
  if (!query) {
    if (canLoadRecommend.value) {
      loadRecommendSongs()
    } else {
      songsList.value = []
    }
    return
  }

  isSearching.value = true
  songsList.value = []
  parsedPlaylistName.value = ''
  selectedSongs.value.clear()

  // 1. 判断是否是链接，链接走 resolve
  const isLink = query.startsWith('http') || query.includes('163.com') || query.includes('163cn.tv')

  try {
    if (isLink) {
      const data = await wsClient.sendRequest('netease/resolve', { input: query })
      if (data) {
        parsedResultType.value = data.type
        parsedPlaylistName.value = data.name
        songsList.value = data.data
        if (songsList.value.length === 0) {
          message.warning('解析完成，但歌曲列表为空')
        } else {
          message.success(`成功解析得到 ${songsList.value.length} 首歌曲`)
        }
      } else {
        message.error('解析链接失败')
      }
    } else {
      // 2. 纯文本走 cloudsearch 检索
      parsedResultType.value = 'search'
      const data = await wsClient.sendRequest('netease/search', { keywords: query, limit: 30 })
      if (data) {
        songsList.value = data
      } else {
        message.error('检索失败')
      }
    }
  } catch (e: any) {
    message.error(e.message || '网易云 API 响应异常')
  } finally {
    isSearching.value = false
  }
}

const toggleSongSelection = (song: NeteaseSong) => {
  if (selectedSongs.value.has(song.id)) {
    selectedSongs.value.delete(song.id)
  } else {
    selectedSongs.value.set(song.id, song)
  }
}

const handleCheckboxChange = (song: NeteaseSong, val: boolean) => {
  if (val) {
    selectedSongs.value.set(song.id, song)
  } else {
    selectedSongs.value.delete(song.id)
  }
}

const isAllSelected = computed(() => {
  return songsList.value.length > 0 && selectedSongs.value.size === songsList.value.length
})

const isSomeSelected = computed(() => {
  return selectedSongs.value.size > 0 && selectedSongs.value.size < songsList.value.length
})

const toggleSelectAll = (checked: boolean) => {
  if (checked) {
    songsList.value.forEach(s => selectedSongs.value.set(s.id, s))
  } else {
    selectedSongs.value.clear()
  }
}

const downloadSingleSong = async (song: NeteaseSong) => {
  const res = await systemStore.startNeteaseDownload(song)
  if (res.success) {
    message.success(`已开始下载《${song.title}》`)
    showTaskDrawer.value = true // 展开抽屉展示进度
  } else {
    message.error(res.error)
  }
}

const downloadSelectedSongs = async () => {
  const list = Array.from(selectedSongs.value.values())
  let startedCount = 0

  for (const song of list) {
    const res = await systemStore.startNeteaseDownload(song)
    if (res.success) {
      startedCount++
    }
  }

  message.success(`成功触发 ${startedCount} 首歌曲并发下载`)
  selectedSongs.value.clear()
  showTaskDrawer.value = true
}

  const showTaskDrawer = ref(false)
const tasksList = computed(() => {
  return Object.values(systemStore.downloadTasks)
})

const runningTasksCount = computed(() => {
  return tasksList.value.filter(t => ['pending', 'preparing', 'downloading'].includes(t.status)).length
})

const totalTasksCount = computed(() => {
  return tasksList.value.length
})

  // 任务排序：下载中 > 排队 > 完成/失败
  const sortedTasks = computed(() => {
  const list = [...tasksList.value]
  const statusWeight = {
    downloading: 0,
    preparing: 1,
    pending: 2,
    error: 3,
    success: 4
  }

  return list.sort((a, b) => {
    return (statusWeight[a.status] || 0) - (statusWeight[b.status] || 0)
  })
})

const formatQuality = (level: string) => {
  const map: Record<string, string> = {
    standard: '标准',
    higher: '较高',
    exhigh: '极高',
    lossless: '无损 SQ',
    hires: 'Hi-Res',
    jyeffect: '高清',
    sky: '沉浸',
    dolby: '杜比',
    jymaster: '超高清'
  }
  return map[level] || level.toUpperCase()
}

const scrollbarWidth = ref(0)
const virtualListRef = ref<any>(null)

const updateScrollbarWidth = () => {
  if (virtualListRef.value?.$el) {
    const el = virtualListRef.value.$el
    scrollbarWidth.value = el.offsetWidth - el.clientWidth
  }
}

watch(() => songsList.value, () => {
  nextTick(() => {
    setTimeout(updateScrollbarWidth, 50)
  })
}, { deep: true })

let unsubscribeLoginStatus: (() => void) | null = null

onMounted(() => {
  systemStore.fetchNeteaseConfig()
  systemStore.fetchNeteaseUserStatus()

  // 初始化 WebSocket 连接
  systemStore.initWebSocket()

  // 订阅网易云扫码登录状态主动推送
  unsubscribeLoginStatus = wsClient.subscribe('netease_login_status', (data) => {
    if (!data || data.key !== qrKey.value) return
    
    const stat = data.status // authorized, scanned, expired, waiting
    if (stat === 'authorized') {
      qrStatus.value = 'success'
      qrStatusText.value = '授权登录成功！'
      message.success('登录成功')
      setTimeout(async () => {
        showQrModal.value = false
        systemStore.clearNeteaseRecommendCache() // 登录成功，清空旧账号的每日推荐缓存
        await systemStore.fetchNeteaseUserStatus()
      }, 1500)
    } else if (stat === 'scanned') {
      qrStatus.value = 'scanned'
      qrStatusText.value = '已扫码，请在手机上点击确认登录'
    } else if (stat === 'expired') {
      qrStatus.value = 'expired'
      qrStatusText.value = '二维码已过期，请重新点击登录获取'
    } else if (stat === 'waiting') {
      qrStatus.value = 'waiting'
      qrStatusText.value = '请打开网易云音乐 APP 扫码登录'
    }
  })

  // 若当前正在部署 Docker，继续拉取
  if (systemStore.dockerInstallStatus.status === 'running') {
    // 部署状态由 Settings 页面管理
  }

  setTimeout(updateScrollbarWidth, 200)
})

onUnmounted(() => {
  if (unsubscribeLoginStatus) {
    unsubscribeLoginStatus()
    unsubscribeLoginStatus = null
  }
})
</script>
