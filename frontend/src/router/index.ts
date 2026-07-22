import { createRouter, createWebHashHistory, type RouteRecordRaw } from 'vue-router'
import LocalMusic from '../views/LocalMusic.vue'
import Favorites from '../views/Favorites.vue'
import PlayHistory from '../views/PlayHistory.vue'
import MountManager from '../views/MountManager.vue'
import NeteaseDownloader from '../views/NeteaseDownloader.vue'
import MusicUploader from '../views/MusicUploader.vue'
import Settings from '../views/Settings.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'LocalMusic',
    component: LocalMusic
  },
  {
    path: '/login',
    name: 'Login',
    redirect: () => {
      window.dispatchEvent(new CustomEvent('2fmusic-unauthorized'))
      return '/'
    }
  },
  {
    path: '/favorites',
    name: 'Favorites',
    component: Favorites
  },
  {
    path: '/favorites/:id',
    name: 'PlaylistDetail',
    component: LocalMusic
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
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
