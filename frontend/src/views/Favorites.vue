<template>
  <div class="flex flex-col">
    <div class="view-header">
      <h1 class="view-title">我的收藏</h1>
      <n-button class="create-playlist-btn max-md:w-[38px] max-md:h-[38px] max-md:p-0 max-md:justify-center max-md:rounded-full [&_.n-button__content]:max-md:hidden! [&_.n-button__icon]:max-md:m-0!" round type="primary" @click="showCreateModal = true">
        <template #icon>
          <SvgIcon name="plus" />
        </template>
        新建收藏夹
      </n-button>
    </div>

    <!-- 收藏夹画廊列表 -->
    <div v-auto-animate class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-8 3xl:grid-cols-10 gap-x-4 gap-y-6">
      <div 
        v-for="playlist in favoritesStore.playlists" 
        :key="playlist.id" 
        class="playlist-card glass-card p-4 rounded-xl cursor-pointer md:p-6 md:rounded-2xl group"
        @click="goToPlaylistDetail(playlist.id)"
      >
        <div class="flex justify-between items-center mb-3 md:mb-5">
          <div class="card-icon-box w-9 h-9 text-sm rounded-lg md:w-11 md:h-11 md:text-lg md:rounded-xl">
            <SvgIcon name="music" />
          </div>
          <n-button 
            v-if="playlist.is_default !== 1" 
            circle 
            text
            type="error"
            class="card-delete-btn opacity-0 transition-opacity duration-150 group-hover:opacity-60 max-md:opacity-85 max-md:bg-danger/4 active:scale-95 hover:opacity-100! hover:text-danger! hover:bg-danger-soft!" 
            @click.stop="confirmDeletePlaylist(playlist)"
            title="删除收藏夹"
          >
            <template #icon><SvgIcon name="trash" /></template>
          </n-button>
        </div>
        
        <div class="flex flex-col">
          <h3 class="text-sm md:text-base font-semibold text-ink m-0 mb-1 truncate">{{ playlist.name }}</h3>
          <p class="text-[11px] md:text-xs text-body-muted m-0">{{ playlist.song_count || 0 }} 首歌曲</p>
        </div>
      </div>
    </div>

    <!-- 新建收藏夹弹窗 -->
    <n-modal v-model:show="showCreateModal" preset="dialog" title="新建收藏夹">
      <div class="flex flex-col gap-2 mt-3">
        <label for="playlist-name-input" class="text-xs font-semibold text-body-muted">收藏夹名称</label>
        <n-input 
          id="playlist-name-input"
          v-model:value="newPlaylistName" 
          placeholder="请输入收藏夹名称..." 
          clearable
          @keyup.enter="handleCreatePlaylist"
        />
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showCreateModal = false">取消</n-button>
          <n-button round type="primary" @click="handleCreatePlaylist" :disabled="!newPlaylistName.trim()">创建</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 删除确认弹窗 -->
    <n-modal v-model:show="showDeleteConfirm" preset="dialog" title="确认删除收藏夹">
      <div>
        确定要删除收藏夹《{{ playlistToDelete?.name }}》吗？删除收藏夹<strong>不会</strong>删除磁盘上的歌曲文件，但会清除该收藏夹内的所有收藏关系。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showDeleteConfirm = false">取消</n-button>
          <n-button round type="error" @click="handleDeletePlaylist">确认删除</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { vAutoAnimate } from '@formkit/auto-animate/vue'
import { useRouter } from 'vue-router'
import { useFavoritesStore } from '../stores/favorites'
import { NModal, NButton, NInput, NSpace, useMessage } from 'naive-ui'
import type { FavoritePlaylist } from '../types'

const router = useRouter()
const favoritesStore = useFavoritesStore()
const message = useMessage()

const showCreateModal = ref(false)
const newPlaylistName = ref('')

const showDeleteConfirm = ref(false)
const playlistToDelete = ref<FavoritePlaylist | null>(null)

const goToPlaylistDetail = (id: string) => {
  router.push(`/favorites/${id}`)
}

const handleCreatePlaylist = async () => {
  const name = newPlaylistName.value.trim()
  if (!name) return

  const res = await favoritesStore.createPlaylist(name)
  if (res.success) {
    message.success(`已创建收藏夹《${name}》`)
    newPlaylistName.value = ''
    showCreateModal.value = false
  } else {
    message.error(res.error)
  }
}

const confirmDeletePlaylist = (playlist: FavoritePlaylist) => {
  playlistToDelete.value = playlist
  showDeleteConfirm.value = true
}

const handleDeletePlaylist = async () => {
  if (!playlistToDelete.value) return
  const id = playlistToDelete.value.id
  const name = playlistToDelete.value.name
  
  const res = await favoritesStore.deletePlaylist(id)
  showDeleteConfirm.value = false
  if (res.success) {
    message.success(`已成功删除收藏夹《${name}》`)
  } else {
    message.error(res.error)
  }
}

onMounted(() => {
  if (localStorage.getItem('2fmusic_password')) {
    favoritesStore.fetchPlaylists()
  }
})
</script>


