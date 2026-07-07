import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import LocalMusic from '../views/LocalMusic.vue'
import Favorites from '../views/Favorites.vue'
import PlayHistory from '../views/PlayHistory.vue'
import MountManager from '../views/MountManager.vue'
import NeteaseDownloader from '../views/NeteaseDownloader.vue'
import MusicUploader from '../views/MusicUploader.vue'
import Settings from '../views/Settings.vue'
import FoliaPlayer from '../views/FoliaPlayer.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'LocalMusic',
    component: LocalMusic
  },
  {
    path: '/favorites',
    name: 'Favorites',
    component: Favorites
  },
  {
    path: '/favorites/:id',
    name: 'PlaylistDetail',
    component: LocalMusic // 路由参数过滤，复用 LocalMusic
  },
  {
    path: '/history',
    name: 'PlayHistory',
    component: PlayHistory
  },
  {
    path: '/mounts',
    name: 'MountManager',
    component: MountManager
  },
  {
    path: '/netease',
    name: 'NeteaseDownloader',
    component: NeteaseDownloader
  },
  {
    path: '/upload',
    name: 'MusicUploader',
    component: MusicUploader
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings
  },
  {
    path: '/folia',
    name: 'FoliaPlayer',
    component: FoliaPlayer
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
